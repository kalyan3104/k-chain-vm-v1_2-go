package vmserver

import (
	"github.com/kalyan3104/k-chain-core-go/core"
	vmcommon "github.com/kalyan3104/k-chain-vm-common-go"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/config"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/mock"
	worldmock "github.com/kalyan3104/k-chain-vm-v1_2-go/mock/world"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/vmhost"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/vmhost/hostCore"
)

type worldDataModel struct {
	ID       string
	Accounts worldmock.AccountMap
}

type world struct {
	id             string
	blockchainHook *worldmock.MockWorld
	vm             vmcommon.VMExecutionHandler
}

func newWorldDataModel(worldID string) *worldDataModel {
	return &worldDataModel{
		ID:       worldID,
		Accounts: worldmock.NewAccountMap(),
	}
}

// newWorld creates a new debugging world
func newWorld(dataModel *worldDataModel) (*world, error) {
	blockchainHook := worldmock.NewMockWorld()
	blockchainHook.AcctMap = dataModel.Accounts

	vm, err := hostCore.NewVMHost(
		blockchainHook,
		getHostParameters(),
	)
	if err != nil {
		return nil, err
	}

	return &world{
		id:             dataModel.ID,
		blockchainHook: blockchainHook,
		vm:             vm,
	}, nil
}

func getHostParameters() *vmhost.VMHostParameters {
	return &vmhost.VMHostParameters{
		VMType:             []byte{5, 0},
		BlockGasLimit:      uint64(10000000),
		GasSchedule:        config.MakeGasMap(1, 1),
		ProtectedKeyPrefix: []byte("E" + "L" + "R" + "O" + "N" + "D"),
		EnableEpochsHandler: &mock.EnableEpochsHandlerStub{
			IsFlagEnabledCalled: func(flag core.EnableEpochFlag) bool {
				return flag == hostCore.SCDeployFlag || flag == hostCore.AheadOfTimeGasUsageFlag || flag == hostCore.RepairCallbackFlag || flag == hostCore.BuiltInFunctionsFlag
			},
		},
	}
}

func (w *world) deploySmartContract(request DeployRequest) *DeployResponse {
	input := w.prepareDeployInput(request)
	log.Trace("w.deploySmartContract()", "input", prettyJson(input))

	vmOutput, err := w.vm.RunSmartContractCreate(input)
	if err == nil {
		w.blockchainHook.UpdateAccounts(vmOutput.OutputAccounts, nil)
	}

	response := &DeployResponse{}
	response.ContractResponseBase = createContractResponseBase(&input.VMInput, vmOutput)
	response.Error = err
	response.ContractAddress = w.blockchainHook.LastCreatedContractAddress
	response.ContractAddressHex = toHex(response.ContractAddress)
	return response
}

func (w *world) upgradeSmartContract(request UpgradeRequest) *UpgradeResponse {
	input := w.prepareUpgradeInput(request)
	log.Trace("w.upgradeSmartContract()", "input", prettyJson(input))

	vmOutput, err := w.vm.RunSmartContractCall(input)
	if err == nil {
		w.blockchainHook.UpdateAccounts(vmOutput.OutputAccounts, nil)
	}

	response := &UpgradeResponse{}
	response.ContractResponseBase = createContractResponseBase(&input.VMInput, vmOutput)
	response.Error = err

	return response
}

func (w *world) runSmartContract(request RunRequest) *RunResponse {
	input := w.prepareCallInput(request)
	log.Trace("w.runSmartContract()", "input", prettyJson(input))

	vmOutput, err := w.vm.RunSmartContractCall(input)
	if err == nil {
		w.blockchainHook.UpdateAccounts(vmOutput.OutputAccounts, nil)
	}

	response := &RunResponse{}
	response.ContractResponseBase = createContractResponseBase(&input.VMInput, vmOutput)
	response.Error = err

	return response
}

func (w *world) querySmartContract(request QueryRequest) *QueryResponse {
	input := w.prepareCallInput(request.RunRequest)
	log.Trace("w.querySmartContract()", "input", prettyJson(input))

	vmOutput, err := w.vm.RunSmartContractCall(input)

	response := &QueryResponse{}
	response.ContractResponseBase = createContractResponseBase(&input.VMInput, vmOutput)
	response.Error = err

	return response
}

func (w *world) createAccount(request CreateAccountRequest) *CreateAccountResponse {
	log.Trace("w.createAccount()", "request", prettyJson(request))

	account := worldmock.Account{
		Address: request.Address,
		Nonce:   request.Nonce,
		Balance: request.BalanceAsBigInt,
	}
	w.blockchainHook.AcctMap.PutAccount(&account)
	return &CreateAccountResponse{Account: &account}
}

func (w *world) toDataModel() *worldDataModel {
	return &worldDataModel{
		ID:       w.id,
		Accounts: w.blockchainHook.AcctMap,
	}
}
