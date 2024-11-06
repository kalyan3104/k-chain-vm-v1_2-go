package delegation

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"strings"
	"testing"

	vmi "github.com/kalyan3104/k-chain-vm-common-go"
	worldhook "github.com/kalyan3104/k-chain-vm-v1_2-go/mock/world"
	am "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarioexec"
	fr "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/fileresolver"
	mj "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/json/model"
	mjparse "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/json/parse"
	mjwrite "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/json/write"
	"github.com/stretchr/testify/require"
)

const (
	UserWithdrawOnly    = "getUserWithdrawOnlyStake"
	UserWaiting         = "getUserWaitingStake"
	UserActive          = "getUserActiveStake"
	UserDeferredPayment = "getUserDeferredPaymentStake"
)

type fuzzDelegationExecutorInitArgs struct {
	serviceFee                  int
	ownerMinStake               int
	minStake                    int
	numBlocksBeforeForceUnstake int
	numBlocksBeforeUnbond       int
	numDelegators               int
	stakePerNode                *big.Int
	numGenesisNodes             int
	totalDelegationCap          *big.Int
}

type fuzzDelegationExecutor struct {
	vmTestExecutor *am.VMTestExecutor
	world          *worldhook.MockWorld
	vm             vmi.VMExecutionHandler
	parser         mjparse.Parser
	txIndex        int

	serviceFee                  int
	numBlocksBeforeForceUnstake int
	numBlocksBeforeUnbond       int
	numDelegators               int
	stakePerNode                *big.Int
	ownerAddress                []byte
	delegationContractAddress   []byte
	auctionMockAddress          []byte
	faucetAddress               []byte
	withdrawTargetAddress       []byte
	stakePurchaseForwardAddress []byte
	numNodes                    int
	totalStakeAdded             *big.Int
	totalStakeWithdrawn         *big.Int
	totalRewards                *big.Int
	generatedScenario           *mj.Scenario
}

func newFuzzDelegationExecutor(fileResolver fr.FileResolver) (*fuzzDelegationExecutor, error) {
	vmTestExecutor, err := am.NewVMTestExecutor("../../scenarioexec")
	if err != nil {
		return nil, err
	}
	vmTestExecutor.SetScenariosGasSchedule(mj.GasScheduleV2)

	parser := mjparse.NewParser(fileResolver)

	return &fuzzDelegationExecutor{
		vmTestExecutor:      vmTestExecutor,
		world:               vmTestExecutor.World,
		vm:                  vmTestExecutor.GetVM(),
		parser:              parser,
		txIndex:             0,
		numNodes:            0,
		totalStakeAdded:     big.NewInt(0),
		totalStakeWithdrawn: big.NewInt(0),
		totalRewards:        big.NewInt(0),
		generatedScenario: &mj.Scenario{
			Name: "fuzz generated",
		},
	}, nil
}

func (pfe *fuzzDelegationExecutor) executeStep(stepSnippet string) error {
	step, err := pfe.parser.ParseScenarioStep(stepSnippet)
	if err != nil {
		return err
	}

	pfe.addStep(step)
	return pfe.vmTestExecutor.ExecuteStep(step)
}

func (pfe *fuzzDelegationExecutor) addStep(step mj.Step) {
	pfe.generatedScenario.Steps = append(pfe.generatedScenario.Steps, step)
}

func (pfe *fuzzDelegationExecutor) saveGeneratedScenario() {
	serialized := mjwrite.ScenarioToJSONString(pfe.generatedScenario)

	err := ioutil.WriteFile("fuzz_gen.scen.json", []byte(serialized), 0644)
	if err != nil {
		fmt.Println(err)
	}
}

func (pfe *fuzzDelegationExecutor) delegatorAddress(delegIndex int) []byte {
	if delegIndex == 0 {
		return pfe.ownerAddress
	}

	return []byte(fmt.Sprintf("delegator %5d               s1", delegIndex))
}

func (pfe *fuzzDelegationExecutor) executeTxStep(stepSnippet string) (*vmi.VMOutput, error) {
	step, err := pfe.parser.ParseScenarioStep(stepSnippet)
	if err != nil {
		return nil, err
	}

	txStep, isTx := step.(*mj.TxStep)
	if !isTx {
		return nil, errors.New("tx step expected")
	}

	pfe.addStep(step)

	return pfe.vmTestExecutor.ExecuteTxStep(txStep)
}

func (pfe *fuzzDelegationExecutor) log(info string, args ...interface{}) {
	fmt.Printf(info+"\n", args...)
}

func (pfe *fuzzDelegationExecutor) addNodes(numNodesToAdd int) error {
	pfe.log("addNodes %d -> %d", numNodesToAdd, pfe.numNodes+numNodesToAdd)

	_, err := pfe.executeTxStep(fmt.Sprintf(`
	{
		"step": "scCall",
		"txId": "%d",
		"tx": {
			"from": "''%s",
			"to": "''%s",
			"value": "0",
			"function": "addNodes",
			"arguments": [
				%s
			],
			"gasLimit": "1,000,000,000",
			"gasPrice": "0"
		},
		"expect": {
			"out": [],
			"status": "",
			"logs": [],
			"gas": "*",
			"refund": "*"
		}
	}`,
		pfe.nextTxIndex(),
		string(pfe.ownerAddress),
		string(pfe.delegationContractAddress),
		blsKeySignatureArgsString(pfe.numNodes, numNodesToAdd),
	))
	pfe.numNodes += numNodesToAdd
	return err
}

func (pfe *fuzzDelegationExecutor) removeNodes(numNodesToRemove int) error {
	pfe.log("removeNodes %d -> %d", numNodesToRemove, pfe.numNodes-numNodesToRemove)

	output, err := pfe.executeTxStep(fmt.Sprintf(`
	{
		"step": "scCall",
		"txId": "%d",
		"tx": {
			"from": "''%s",
			"to": "''%s",
			"value": "0",
			"function": "removeNodes",
			"arguments": [
				%s
			],
			"gasLimit": "1,000,000,000",
			"gasPrice": "0"
		}
	}`,
		pfe.nextTxIndex(),
		string(pfe.ownerAddress),
		string(pfe.delegationContractAddress),
		blsKeysToBeRemoved(pfe.numNodes, numNodesToRemove),
	))
	if err != nil {
		return err
	}

	if output.ReturnCode != vmi.Ok {
		pfe.log("could not remove node because %s", output.ReturnMessage)
		return nil
	}

	return nil
}

func (pfe *fuzzDelegationExecutor) nextTxIndex() int {
	pfe.txIndex++
	return pfe.txIndex
}

func blsKeysToBeRemoved(totalNumNodes, numKeysToBeRemoved int) string {
	var blsKeys []string
	for i := 0; i < numKeysToBeRemoved; i++ {
		keyIndex := rand.Intn(totalNumNodes + 1)
		blsKeys = append(blsKeys, "\"''"+blsKey(keyIndex)+"\"")
	}
	return strings.Join(blsKeys, ",")
}

func blsKeySignatureArgsString(startIndex, numNodes int) string {
	var blsKeyArgs []string
	for i := startIndex; i < startIndex+numNodes; i++ {
		blsKeyArgs = append(blsKeyArgs, "\"''"+blsKey(i)+"\"")
		blsKeyArgs = append(blsKeyArgs, "\"''"+blsSignature(i)+"\"")
	}
	return strings.Join(blsKeyArgs, ",")
}

func blsKey(index int) string {
	return fmt.Sprintf(
		"bls key %5d ..................................................................................",
		index)
}

func blsSignature(index int) string {
	return fmt.Sprintf(
		"bls key signature %5d ........................",
		index)
}

func (pfe *fuzzDelegationExecutor) increaseBlockNonce(nonceDelta int) error {
	curentBlockNonce := uint64(0)
	if pfe.world.CurrentBlockInfo != nil {
		curentBlockNonce = pfe.world.CurrentBlockInfo.BlockNonce
	}

	err := pfe.executeStep(fmt.Sprintf(`
	{
		"step": "setState",
		"comment": "%d - increase block nonce",
		"currentBlockInfo": {
			"blockNonce": "%d"
		}
	}`,
		pfe.nextTxIndex(),
		curentBlockNonce+uint64(nonceDelta),
	))
	if err != nil {
		return err
	}

	pfe.log("block nonce: %d ---> %d", curentBlockNonce, curentBlockNonce+uint64(nonceDelta))
	return nil
}

func (pfe *fuzzDelegationExecutor) simpleQuery(funcName string) (*big.Int, error) {
	return pfe.querySingleResult(funcName, "")
}

func (pfe *fuzzDelegationExecutor) querySingleResult(funcName string, args string) (*big.Int, error) {
	output, err := pfe.executeTxStep(fmt.Sprintf(`
	{
		"step": "scCall",
		"txId": "%d",
		"tx": {
			"from": "''%s",
			"to": "''%s",
			"value": "0",
			"function": "%s",
			"arguments": [
				%s
			],
			"gasLimit": "10,000,000",
			"gasPrice": "0"
		},
		"expect": {
			"out": [ "*" ],
			"status": "",
			"logs": [],
			"gas": "*",
			"refund": "*"
		}
	}`,
		pfe.nextTxIndex(),
		string(pfe.ownerAddress),
		string(pfe.delegationContractAddress),
		funcName,
		args,
	))
	if err != nil {
		return nil, err
	}

	result := big.NewInt(0).SetBytes(output.ReturnData[0])
	pfe.log("query: %s -> %d", funcName, result)
	return result, nil
}

func (pfe *fuzzDelegationExecutor) delegatorQuery(funcName string, delegatorIndex int) (*big.Int, error) {
	delegatorAddr := fmt.Sprintf(`"''%s"`, string(pfe.delegatorAddress(delegatorIndex)))
	return pfe.querySingleResult(funcName, delegatorAddr)
}

func (pfe *fuzzDelegationExecutor) getAllDelegatorsBalance() *big.Int {
	totalDelegatorBalance := big.NewInt(0)
	for delegatorIdx := 0; delegatorIdx <= pfe.numDelegators; delegatorIdx++ {
		balance := pfe.getDelegatorBalance(delegatorIdx)
		totalDelegatorBalance.Add(totalDelegatorBalance, balance)
	}

	return totalDelegatorBalance
}

func (pfe *fuzzDelegationExecutor) getDelegatorBalance(delegatorIndex int) *big.Int {
	delegatorAddr := pfe.delegatorAddress(delegatorIndex)
	acct := pfe.world.AcctMap.GetAccount(delegatorAddr)

	return acct.Balance
}

func (pfe *fuzzDelegationExecutor) getAuctionBalance() *big.Int {
	acct := pfe.world.AcctMap.GetAccount(pfe.auctionMockAddress)
	return acct.Balance
}

func (pfe *fuzzDelegationExecutor) getWithdrawTargetBalance() *big.Int {
	acct := pfe.world.AcctMap.GetAccount(pfe.withdrawTargetAddress)
	return acct.Balance
}

func (pfe *fuzzDelegationExecutor) modifyDelegationCap(newCap *big.Int) error {
	output, err := pfe.executeTxStep(fmt.Sprintf(`
	{
		"step": "scCall",
		"txId": "-modify-delegation-cap-",
		"tx": {
			"from": "''%s",
			"to": "''%s",
			"value": "0",
			"function": "modifyTotalDelegationCap",
			"arguments": ["%d"],
			"gasLimit": "100,000,000",
			"gasPrice": "0"
		}
	}`,
		string(pfe.ownerAddress),
		string(pfe.delegationContractAddress),
		newCap,
	))
	if err != nil {
		return err
	}

	pfe.log("modify delegation cap: returned code %s, returned message %s, newDelegationCap %d", output.ReturnCode, output.ReturnMessage, newCap)

	return nil
}

func (pfe *fuzzDelegationExecutor) setServiceFee(newServiceFee int) error {
	output, err := pfe.executeTxStep(fmt.Sprintf(`
	{
		"step": "scCall",
		"txId": "-set-service-fee-",
		"tx": {
			"from": "''%s",
			"to": "''%s",
			"value": "0",
			"function": "setServiceFee",
			"arguments": ["%d"],
			"gasLimit": "100,000,000",
			"gasPrice": "0"
		}
	}`,
		string(pfe.ownerAddress),
		string(pfe.delegationContractAddress),
		newServiceFee,
	))
	if err != nil {
		return err
	}

	pfe.log("modify service fee: returned code %s, newServiceFee %d", output.ReturnCode, newServiceFee)

	return nil
}

func (pfe *fuzzDelegationExecutor) continueGlobalOperation() error {
	completed := false
	for !completed {
		output, err := pfe.executeTxStep(fmt.Sprintf(`
		{
			"step": "scCall",
			"txId": "-continue-global-operation-",
			"tx": {
				"from": "''%s",
				"to": "''%s",
				"value": "0",
				"function": "continueGlobalOperation",
				"arguments": [],
				"gasLimit": "200,000,000",
				"gasPrice": "0"
			},
			"expect": {
				"out": [ "*" ],
				"refund": "*"
			}
		}`,
			string(pfe.ownerAddress),
			string(pfe.delegationContractAddress),
		))
		if err != nil {
			return err
		}
		pfe.log("continue global operation %x", output.ReturnData[0])

		if bytes.Equal(output.ReturnData[0], []byte("completed")) {
			completed = true
		} else if bytes.Equal(output.ReturnData[0], []byte("interrupted")) {
			completed = false
		} else {
			return fmt.Errorf("unexpected global operation status: %x", output.ReturnData[0])
		}
	}

	return nil
}

func (pfe *fuzzDelegationExecutor) getContractBalance() *big.Int {
	acct := pfe.world.AcctMap.GetAccount(pfe.delegationContractAddress)
	return acct.Balance
}

func (pfe *fuzzDelegationExecutor) isBootstrapMode() (bool, error) {
	output, err := pfe.executeTxStep(fmt.Sprintf(`
	{
		"step": "scCall",
		"txId": "-is-bootstrap-mode-",
		"tx": {
			"from": "''%s",
			"to": "''%s",
			"value": "0",
			"function": "isBootstrapMode",
			"arguments": [],
			"gasLimit": "100,000",
			"gasPrice": "0"
		}
	}`,
		string(pfe.ownerAddress),
		string(pfe.delegationContractAddress),
	))
	if err != nil {
		return false, err
	}

	if bytes.Equal(output.ReturnData[0], []byte{1}) {
		return true, nil
	} else {
		return false, nil
	}
}

func (pfe *fuzzDelegationExecutor) printServiceFeeAndDelegationCap(t *testing.T) {
	_, err := pfe.querySingleResult("getTotalDelegationCap", "")
	require.Nil(t, err)

	_, err = pfe.querySingleResult("getServiceFee", "")
	require.Nil(t, err)
}
