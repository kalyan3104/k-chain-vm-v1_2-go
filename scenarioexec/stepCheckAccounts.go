package scenarioexec

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/kalyan3104/k-chain-core-go/data/dcdt"
	worldmock "github.com/kalyan3104/k-chain-vm-v1_2-go/mock/world"
	er "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/expression/reconstructor"
	mj "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/json/model"
	oj "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/orderedjson"
)

// ExecuteCheckStateStep executes a CheckStateStep defined by the current scenario.
func (ae *VMTestExecutor) ExecuteCheckStateStep(step *mj.CheckStateStep) error {
	if len(step.Comment) > 0 {
		log.Trace("CheckStateStep", "comment", step.Comment)
	}

	return ae.checkAccounts(step.CheckAccounts)
}

func (ae *VMTestExecutor) checkAccounts(checkAccounts *mj.CheckAccounts) error {
	if !checkAccounts.OtherAccountsAllowed {
		for worldAcctAddr := range ae.World.AcctMap {
			postAcctMatch := mj.FindCheckAccount(checkAccounts.Accounts, []byte(worldAcctAddr))
			if postAcctMatch == nil {
				return fmt.Errorf("unexpected account address: %s",
					ae.exprReconstructor.Reconstruct(
						[]byte(worldAcctAddr),
						er.AddressHint))
			}
		}
	}

	for _, expectedAcct := range checkAccounts.Accounts {
		matchingAcct, isMatch := ae.World.AcctMap[string(expectedAcct.Address.Value)]
		if !isMatch {
			return fmt.Errorf("account %s expected but not found after running test",
				ae.exprReconstructor.Reconstruct(
					expectedAcct.Address.Value,
					er.AddressHint))
		}

		if !bytes.Equal(matchingAcct.Address, expectedAcct.Address.Value) {
			return fmt.Errorf("bad account address %s",
				ae.exprReconstructor.Reconstruct(
					matchingAcct.Address,
					er.AddressHint))
		}

		if !expectedAcct.Nonce.Check(matchingAcct.Nonce) {
			return fmt.Errorf("bad account nonce. Account: %s. Want: \"%s\". Have: %d",
				hex.EncodeToString(matchingAcct.Address),
				expectedAcct.Nonce.Original,
				matchingAcct.Nonce)
		}

		if !expectedAcct.Balance.Check(matchingAcct.Balance) {
			return fmt.Errorf("bad account balance. Account: %s. Want: \"%s\". Have: \"%s\"",
				hex.EncodeToString(matchingAcct.Address),
				expectedAcct.Balance.Original,
				ae.exprReconstructor.ReconstructFromBigInt(matchingAcct.Balance))
		}

		if !expectedAcct.Username.Check(matchingAcct.Username) {
			return fmt.Errorf("bad account username. Account: %s. Want: %s. Have: \"%s\"",
				hex.EncodeToString(matchingAcct.Address),
				oj.JSONString(expectedAcct.Username.Original),
				ae.exprReconstructor.Reconstruct(
					matchingAcct.Username,
					er.StrHint))
		}

		if !expectedAcct.Code.Check(matchingAcct.Code) {
			return fmt.Errorf("bad account code. Account: %s. Want: [%s]. Have: [%s]",
				hex.EncodeToString(matchingAcct.Address),
				expectedAcct.Code.Original,
				string(matchingAcct.Code))
		}

		// currently ignoring asyncCallData that is unspecified in the json
		if !expectedAcct.AsyncCallData.IsUnspecified() &&
			!expectedAcct.AsyncCallData.Check([]byte(matchingAcct.AsyncCallData)) {
			return fmt.Errorf("bad async call data. Account: %s. Want: [%s]. Have: [%s]",
				hex.EncodeToString(matchingAcct.Address),
				expectedAcct.AsyncCallData.Original,
				matchingAcct.AsyncCallData)
		}

		err := ae.checkAccountStorage(expectedAcct, matchingAcct)
		if err != nil {
			return err
		}

		err = ae.checkAccountDCDT(expectedAcct, matchingAcct)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ae *VMTestExecutor) checkAccountStorage(expectedAcct *mj.CheckAccount, matchingAcct *worldmock.Account) error {
	if expectedAcct.IgnoreStorage {
		return nil
	}

	expectedStorage := make(map[string][]byte)
	for _, stkvp := range expectedAcct.CheckStorage {
		expectedStorage[string(stkvp.Key.Value)] = stkvp.Value.Value
	}

	allKeys := make(map[string]bool)
	for k := range expectedStorage {
		allKeys[k] = true
	}
	for k := range matchingAcct.Storage {
		allKeys[k] = true
	}
	storageError := ""
	for k := range allKeys {
		want := expectedStorage[k]
		have := matchingAcct.StorageValue(k)
		if !bytes.Equal(want, have) && !worldmock.IsDCDTKey([]byte(k)) {
			storageError += fmt.Sprintf(
				"\n  for key %s: Want: %s. Have: %s",
				ae.exprReconstructor.Reconstruct([]byte(k), er.NoHint),
				ae.exprReconstructor.Reconstruct(want, er.NoHint),
				ae.exprReconstructor.Reconstruct(have, er.NoHint))
		}
	}
	if len(storageError) > 0 {
		return fmt.Errorf("wrong account storage for account \"%s\":%s",
			expectedAcct.Address.Original, storageError)
	}
	return nil
}

func (ae *VMTestExecutor) checkAccountDCDT(expectedAcct *mj.CheckAccount, matchingAcct *worldmock.Account) error {
	if expectedAcct.IgnoreDCDT {
		return nil
	}

	accountAddress := expectedAcct.Address.Original
	expectedTokens := getExpectedTokens(expectedAcct)
	accountTokens, err := matchingAcct.GetFullMockDCDTData()
	if err != nil {
		return err
	}

	allTokenNames := make(map[string]bool)
	for tokenName := range expectedTokens {
		allTokenNames[tokenName] = true
	}
	for tokenName := range accountTokens {
		allTokenNames[tokenName] = true
	}
	var errors []error
	for tokenName := range allTokenNames {
		expectedToken := expectedTokens[tokenName]
		accountToken := accountTokens[tokenName]
		if expectedToken == nil {
			expectedToken = &mj.CheckDCDTData{
				TokenIdentifier: mj.JSONBytesFromString{
					Value:    []byte(tokenName),
					Original: ae.exprReconstructor.Reconstruct([]byte(tokenName), er.StrHint),
				},
				Instances: nil,
				LastNonce: mj.JSONCheckUint64{Value: 0, Original: ""},
				Roles:     nil,
			}
		} else if accountToken == nil {
			accountToken = &worldmock.MockDCDTData{
				TokenIdentifier: []byte(tokenName),
				Instances:       nil,
				LastNonce:       0,
				Roles:           nil,
			}
		} else {
			errors = append(errors, checkTokenState(accountAddress, tokenName, expectedToken, accountToken)...)
		}
	}

	errorString := makeErrorString(errors)
	if len(errorString) > 0 {
		return fmt.Errorf("mismatch for account %s: %s", accountAddress, errorString)
	}

	return nil
}

func getExpectedTokens(expectedAcct *mj.CheckAccount) map[string]*mj.CheckDCDTData {
	expectedTokens := make(map[string]*mj.CheckDCDTData)
	for _, expectedTokenData := range expectedAcct.CheckDCDTData {
		tokenName := expectedTokenData.TokenIdentifier.Value
		expectedTokens[string(tokenName)] = expectedTokenData
	}

	return expectedTokens
}

func checkTokenState(
	accountAddress string,
	tokenName string,
	expectedToken *mj.CheckDCDTData,
	accountToken *worldmock.MockDCDTData) []error {

	var errors []error

	errors = append(errors, checkTokenInstances(accountAddress, tokenName, expectedToken, accountToken)...)

	if !expectedToken.LastNonce.Check(accountToken.LastNonce) {
		errors = append(errors, fmt.Errorf("bad account DCDT last nonce. Account: %s. Token: %s. Want: \"%s\". Have: %d",
			accountAddress,
			tokenName,
			expectedToken.LastNonce.Original,
			accountToken.LastNonce))
	}

	errors = append(errors, checkTokenRoles(accountAddress, tokenName, expectedToken, accountToken)...)

	return errors
}

func checkTokenInstances(
	accountAddress string,
	tokenName string,
	expectedToken *mj.CheckDCDTData,
	accountToken *worldmock.MockDCDTData) []error {

	var errors []error

	allNonces := make(map[uint64]bool)
	expectedInstances := make(map[uint64]*mj.CheckDCDTInstance)
	accountInstances := make(map[uint64]*dcdt.DCDigitalToken)
	for _, expectedInstance := range expectedToken.Instances {
		nonce := expectedInstance.Nonce.Value
		allNonces[nonce] = true
		expectedInstances[nonce] = expectedInstance
	}
	for _, accountInstance := range accountToken.Instances {
		nonce := accountInstance.TokenMetaData.Nonce
		allNonces[nonce] = true
		accountInstances[nonce] = accountInstance
	}

	for nonce := range allNonces {
		expectedInstance := expectedInstances[nonce]
		accountInstance := accountInstances[nonce]

		if expectedInstance == nil {
			expectedInstance = &mj.CheckDCDTInstance{
				Nonce:   mj.JSONCheckUint64{Value: nonce, Original: ""},
				Balance: mj.JSONCheckBigInt{Value: big.NewInt(0), Original: ""},
			}
		} else if accountInstance == nil {
			accountInstance = &dcdt.DCDigitalToken{
				Value: big.NewInt(0),
				TokenMetaData: &dcdt.MetaData{
					Name:  []byte(tokenName),
					Nonce: nonce,
				},
			}
		} else {
			if !expectedInstance.Balance.Check(accountInstance.Value) {
				errors = append(errors, fmt.Errorf("bad DCDT balance. Account: %s. Token: %s. Nonce: %d. Want: %s. Have: %d",
					accountAddress,
					tokenName,
					nonce,
					expectedInstance.Balance.Original,
					accountInstance.Value))
			}

			// TODO: check metadata/properties
		}
	}

	return errors
}

func checkTokenRoles(
	accountAddress string,
	tokenName string,
	expectedToken *mj.CheckDCDTData,
	accountToken *worldmock.MockDCDTData) []error {

	var errors []error

	allRoles := make(map[string]bool)
	expectedRoles := make(map[string]bool)
	accountRoles := make(map[string]bool)

	for _, expectedRole := range expectedToken.Roles {
		allRoles[expectedRole] = true
		expectedRoles[expectedRole] = true
	}
	for _, accountRole := range accountToken.Roles {
		allRoles[string(accountRole)] = true
		accountRoles[string(accountRole)] = true
	}
	for role := range allRoles {
		if !expectedRoles[role] {
			errors = append(errors, fmt.Errorf("unexpected DCDT role. Account: %s. Token: %s. Role: %s",
				accountAddress,
				tokenName,
				role))
		}
		if !accountRoles[role] {
			errors = append(errors, fmt.Errorf("missing DCDT role. Account: %s. Token: %s. Role: %s",
				accountAddress,
				tokenName,
				role))
		}
	}

	return errors
}

func makeErrorString(errors []error) string {
	errorString := ""
	for i, err := range errors {
		errorString += err.Error()
		if i < len(errors)-1 {
			errorString += "\n"
		}
	}
	return errorString
}
