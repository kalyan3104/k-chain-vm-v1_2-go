package hostCore

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/kalyan3104/k-chain-core-go/core"
	"github.com/kalyan3104/k-chain-core-go/core/check"
	"github.com/kalyan3104/k-chain-core-go/data/vm"
	vmcommon "github.com/kalyan3104/k-chain-vm-common-go"
	"github.com/kalyan3104/k-chain-vm-common-go/parsers"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/math"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/vmhost"
)

func (host *vmHost) doRunSmartContractCreate(input *vmcommon.ContractCreateInput) *vmcommon.VMOutput {
	host.InitState()
	defer host.Clean()

	_, blockchain, metering, output, runtime, storage := host.GetContexts()

	address, err := blockchain.NewAddress(input.CallerAddr)
	if err != nil {
		return output.CreateVMOutputInCaseOfError(err)
	}

	runtime.SetVMInput(&input.VMInput)
	runtime.SetSCAddress(address)
	metering.InitStateFromContractCallInput(&input.VMInput)

	output.AddTxValueToAccount(address, input.CallValue)
	storage.SetAddress(runtime.GetSCAddress())

	codeDeployInput := vmhost.CodeDeployInput{
		ContractCode:         input.ContractCode,
		ContractCodeMetadata: input.ContractCodeMetadata,
		ContractAddress:      address,
		CodeDeployerAddress:  input.CallerAddr,
	}

	vmOutput, err := host.performCodeDeployment(codeDeployInput)
	if err != nil {
		log.Trace("doRunSmartContractCreate", "error", err)
		return output.CreateVMOutputInCaseOfError(err)
	}

	log.Trace("doRunSmartContractCreate",
		"retCode", vmOutput.ReturnCode,
		"message", vmOutput.ReturnMessage,
		"data", vmOutput.ReturnData)

	return vmOutput
}

func (host *vmHost) performCodeDeployment(input vmhost.CodeDeployInput) (*vmcommon.VMOutput, error) {
	log.Trace("performCodeDeployment", "address", input.ContractAddress, "len(code)", len(input.ContractCode), "metadata", input.ContractCodeMetadata)

	_, _, metering, output, runtime, _ := host.GetContexts()

	err := metering.DeductInitialGasForDirectDeployment(input)
	if err != nil {
		output.SetReturnCode(vmcommon.OutOfGas)
		return nil, err
	}

	runtime.MustVerifyNextContractCode()

	err = runtime.StartWasmerInstance(input.ContractCode, metering.GetGasForExecution(), true)
	if err != nil {
		log.Debug("performCodeDeployment/StartWasmerInstance", "err", err)
		return nil, vmhost.ErrContractInvalid
	}

	err = host.callInitFunction()
	if err != nil {
		return nil, err
	}

	output.DeployCode(input)
	vmOutput := output.GetVMOutput()
	runtime.CleanWasmerInstance()
	return vmOutput, nil
}

// doRunSmartContractUpgrade upgrades a contract directly
func (host *vmHost) doRunSmartContractUpgrade(input *vmcommon.ContractCallInput) *vmcommon.VMOutput {
	host.InitState()
	defer host.Clean()

	_, _, metering, output, runtime, storage := host.GetContexts()

	runtime.InitStateFromContractCallInput(input)
	metering.InitStateFromContractCallInput(&input.VMInput)
	output.AddTxValueToAccount(input.RecipientAddr, input.CallValue)
	storage.SetAddress(runtime.GetSCAddress())

	code, codeMetadata, err := runtime.ExtractCodeUpgradeFromArgs()
	if err != nil {
		return output.CreateVMOutputInCaseOfError(vmhost.ErrInvalidUpgradeArguments)
	}

	codeDeployInput := vmhost.CodeDeployInput{
		ContractCode:         code,
		ContractCodeMetadata: codeMetadata,
		ContractAddress:      input.RecipientAddr,
		CodeDeployerAddress:  input.CallerAddr,
	}

	vmOutput, err := host.performCodeDeployment(codeDeployInput)
	if err != nil {
		log.Trace("doRunSmartContractUpgrade", "error", err)
		return output.CreateVMOutputInCaseOfError(err)
	}

	return vmOutput
}

func (host *vmHost) checkGasForGetCode(input *vmcommon.ContractCallInput, metering vmhost.MeteringContext) error {
	if !host.IsVMV2Enabled() {
		return nil
	}

	getCodeBaseCost := metering.GasSchedule().BaseOperationCost.GetCode
	if input.GasProvided < getCodeBaseCost {
		return vmhost.ErrNotEnoughGas
	}

	return nil
}

func (host *vmHost) doRunSmartContractCall(input *vmcommon.ContractCallInput) (vmOutput *vmcommon.VMOutput) {
	host.InitState()
	defer host.Clean()

	_, _, metering, output, runtime, storage := host.GetContexts()

	runtime.InitStateFromContractCallInput(input)
	metering.InitStateFromContractCallInput(&input.VMInput)
	output.AddTxValueToAccount(input.RecipientAddr, input.CallValue)
	storage.SetAddress(runtime.GetSCAddress())

	err := host.checkGasForGetCode(input, metering)
	if err != nil {
		log.Trace("doRunSmartContractCall get code", "error", vmhost.ErrNotEnoughGas)
		return output.CreateVMOutputInCaseOfError(vmhost.ErrNotEnoughGas)
	}

	contract, err := runtime.GetSCCode()
	if err != nil {
		log.Trace("doRunSmartContractCall get code", "error", vmhost.ErrContractNotFound)
		return output.CreateVMOutputInCaseOfError(vmhost.ErrContractNotFound)
	}

	err = metering.DeductInitialGasForExecution(contract)
	if err != nil {
		log.Trace("doRunSmartContractCall initial gas", "error", vmhost.ErrNotEnoughGas)
		return output.CreateVMOutputInCaseOfError(vmhost.ErrNotEnoughGas)
	}

	err = runtime.StartWasmerInstance(contract, metering.GetGasForExecution(), false)
	if err != nil {
		return output.CreateVMOutputInCaseOfError(vmhost.ErrContractInvalid)
	}

	err = host.callSCMethod()
	if err != nil {
		log.Trace("doRunSmartContractCall", "error", err)
		return output.CreateVMOutputInCaseOfError(err)
	}

	vmOutput = output.GetVMOutput()

	log.Trace("doRunSmartContractCall finished",
		"retCode", vmOutput.ReturnCode,
		"message", vmOutput.ReturnMessage,
		"data", vmOutput.ReturnData)

	runtime.CleanWasmerInstance()
	return
}

func copyTxHashesFromContext(copyEnabled bool, runtime vmhost.RuntimeContext, input *vmcommon.ContractCallInput) {
	if !copyEnabled {
		return
	}
	currentVMInput := runtime.GetVMInput()
	if len(currentVMInput.OriginalTxHash) > 0 {
		input.OriginalTxHash = currentVMInput.OriginalTxHash
	}
	if len(currentVMInput.CurrentTxHash) > 0 {
		input.CurrentTxHash = currentVMInput.CurrentTxHash
	}
	if len(currentVMInput.PrevTxHash) > 0 {
		input.PrevTxHash = currentVMInput.PrevTxHash
	}

}

// ExecuteOnDestContext pushes each context to the corresponding stack
// and initializes new contexts for executing the contract call with the given input
func (host *vmHost) ExecuteOnDestContext(input *vmcommon.ContractCallInput) (vmOutput *vmcommon.VMOutput, asyncInfo *vmhost.AsyncContextInfo, gasUsedBeforeReset uint64, err error) {
	log.Trace("ExecuteOnDestContext", "caller", input.CallerAddr, "dest", input.RecipientAddr, "function", input.Function)

	bigInt, _, metering, output, runtime, storage := host.GetContexts()

	bigInt.PushState()
	bigInt.InitState()

	output.PushState()
	output.CensorVMOutput()

	copyTxHashesFromContext(host.IsDCDTFunctionsEnabled(), runtime, input)
	runtime.PushState()
	runtime.InitStateFromContractCallInput(input)

	metering.PushState()
	metering.InitStateFromContractCallInput(&input.VMInput)
	host.computeGasUsedBefore()

	storage.PushState()
	storage.SetAddress(runtime.GetSCAddress())

	defer func() {
		vmOutput = host.finishExecuteOnDestContext(err)
		metering.SetTotalUsedGas(0)

		if err == nil && vmOutput.ReturnCode != vmcommon.Ok {
			err = vmhost.ErrExecutionFailed
		}
	}()

	// Perform a value transfer to the called SC. If the execution fails, this
	// transfer will not persist.
	if input.CallType != vm.AsynchronousCallBack || input.CallValue.Cmp(vmhost.Zero) == 0 {
		err = output.TransferValueOnly(input.RecipientAddr, input.CallerAddr, input.CallValue, false)
		if err != nil {
			log.Trace("ExecuteOnDestContext transfer", "error", err)
			return
		}
	}

	gasUsedBeforeReset, err = host.execute(input)
	if err != nil {
		log.Trace("ExecuteOnDestContext execution", "error", err)
		return
	}

	asyncInfo = runtime.GetAsyncContextInfo()
	_, err = host.processAsyncInfo(asyncInfo)
	return
}

func (host *vmHost) finishExecuteOnDestContext(executeErr error) *vmcommon.VMOutput {
	bigInt, _, metering, output, runtime, storage := host.GetContexts()

	var vmOutput *vmcommon.VMOutput
	if executeErr != nil {
		// Execution failed: restore contexts as if the execution didn't happen,
		// but first create a vmOutput to capture the error.
		vmOutput = output.CreateVMOutputInCaseOfError(executeErr)
	} else {
		// Retrieve the VMOutput before popping the Runtime state and the previous
		// instance, to ensure accurate GasRemaining
		vmOutput = output.GetVMOutput()
	}

	childContract := runtime.GetSCAddress()
	gasSpentByChildContract := metering.GasSpentByContract()

	if vmOutput.ReturnCode != vmcommon.Ok {
		gasSpentByChildContract = 0
	}

	// Restore the previous context states, except Output, which will be merged
	// into the initial state (VMOutput), but only if it the child execution
	// returned vmcommon.Ok.
	bigInt.PopSetActiveState()
	metering.PopSetActiveState()
	runtime.PopSetActiveState()
	storage.PopSetActiveState()

	// Restore remaining gas to the caller Wasmer instance
	metering.RestoreGas(vmOutput.GasRemaining)
	metering.ForwardGas(runtime.GetSCAddress(), childContract, gasSpentByChildContract)

	if vmOutput.ReturnCode == vmcommon.Ok {
		output.PopMergeActiveState()
	} else {
		output.PopSetActiveState()
	}

	log.Trace("ExecuteOnDestContext finished", "gas spent", gasSpentByChildContract)

	return vmOutput
}

// ExecuteOnSameContext executes the contract call with the given input
// on the same runtime context. Some other contexts are backed up.
func (host *vmHost) ExecuteOnSameContext(input *vmcommon.ContractCallInput) (asyncInfo *vmhost.AsyncContextInfo, err error) {
	log.Trace("ExecuteOnSameContext", "function", input.Function)

	if host.IsBuiltinFunctionName(input.Function) {
		return nil, vmhost.ErrBuiltinCallOnSameContextDisallowed
	}

	bigInt, _, metering, output, runtime, _ := host.GetContexts()

	// Back up the states of the contexts (except Storage, which isn't affected
	// by ExecuteOnSameContext())
	bigInt.PushState()
	output.PushState()

	copyTxHashesFromContext(host.IsDCDTFunctionsEnabled(), runtime, input)
	runtime.PushState()
	runtime.InitStateFromContractCallInput(input)

	metering.PushState()
	metering.InitStateFromContractCallInput(&input.VMInput)

	defer func() {
		host.finishExecuteOnSameContext(err)
	}()

	// Perform a value transfer to the called SC. If the execution fails, this
	// transfer will not persist.
	err = output.TransferValueOnly(input.RecipientAddr, input.CallerAddr, input.CallValue, false)
	if err != nil {
		return
	}

	_, err = host.execute(input)
	if err != nil {
		return
	}

	asyncInfo = runtime.GetAsyncContextInfo()
	return
}

func (host *vmHost) finishExecuteOnSameContext(executeErr error) {
	bigInt, _, metering, output, runtime, _ := host.GetContexts()

	if output.ReturnCode() != vmcommon.Ok || executeErr != nil {
		// Execution failed: restore contexts as if the execution didn't happen.
		bigInt.PopSetActiveState()
		metering.PopSetActiveState()
		output.PopSetActiveState()
		runtime.PopSetActiveState()

		return
	}

	childContract := runtime.GetSCAddress()

	// Retrieve the VMOutput before popping the Runtime state and the previous
	// instance, to ensure accurate GasRemaining
	vmOutput := output.GetVMOutput()
	gasSpentByContract := metering.GasSpentByContract()
	if vmOutput.ReturnCode != vmcommon.Ok {
		gasSpentByContract = 0
	}

	// Execution successful: discard the backups made at the beginning and
	// resume from the new state. However, output.PopDiscard() will ensure that
	// all GasUsed records will be restored, undoing the action of output.ResetGas()
	bigInt.PopDiscard()
	output.PopDiscard()
	metering.PopSetActiveState()
	runtime.PopSetActiveState()

	// Restore remaining gas to the caller Wasmer instance
	metering.RestoreGas(vmOutput.GasRemaining)
	metering.ForwardGas(runtime.GetSCAddress(), childContract, gasSpentByContract)
}

func (host *vmHost) isInitFunctionBeingCalled() bool {
	functionName := host.Runtime().Function()
	return functionName == vmhost.InitFunctionName || functionName == vmhost.InitFunctionNameEth
}

func (host *vmHost) isBuiltinFunctionBeingCalled() bool {
	functionName := host.Runtime().Function()
	return host.IsBuiltinFunctionName(functionName)
}

// IsBuiltinFunctionName returns true if the given function name is the same as any protocol builtin function
func (host *vmHost) IsBuiltinFunctionName(functionName string) bool {
	_, ok := host.protocolBuiltinFunctions[functionName]
	return ok
}

// CreateNewContract creates a new contract indirectly (from another Smart Contract)
func (host *vmHost) CreateNewContract(input *vmcommon.ContractCreateInput) (newContractAddress []byte, err error) {
	newContractAddress = nil
	err = nil

	defer func() {
		if err != nil {
			newContractAddress = nil
		}
	}()

	_, blockchain, metering, output, runtime, _ := host.GetContexts()

	codeDeployInput := vmhost.CodeDeployInput{
		ContractCode:         input.ContractCode,
		ContractCodeMetadata: input.ContractCodeMetadata,
		ContractAddress:      nil,
		CodeDeployerAddress:  input.CallerAddr,
	}
	err = metering.DeductInitialGasForIndirectDeployment(codeDeployInput)
	if err != nil {
		return
	}

	if runtime.ReadOnly() {
		err = vmhost.ErrInvalidCallOnReadOnlyMode
		return
	}

	newContractAddress, err = blockchain.NewAddress(input.CallerAddr)
	if err != nil {
		return
	}

	if blockchain.AccountExists(newContractAddress) {
		err = vmhost.ErrDeploymentOverExistingAccount
		return
	}

	codeDeployInput.ContractAddress = newContractAddress
	output.DeployCode(codeDeployInput)

	defer func() {
		if err != nil {
			output.DeleteOutputAccount(newContractAddress)
		}
	}()

	runtime.MustVerifyNextContractCode()

	initCallInput := &vmcommon.ContractCallInput{
		RecipientAddr:     newContractAddress,
		Function:          vmhost.InitFunctionName,
		AllowInitFunction: true,
		VMInput:           input.VMInput,
	}
	_, _, _, err = host.ExecuteOnDestContext(initCallInput)
	if err != nil {
		return
	}

	blockchain.IncreaseNonce(input.CallerAddr)

	return
}

func (host *vmHost) checkUpgradePermission(vmInput *vmcommon.ContractCallInput) error {
	contract, err := host.blockChainHook.GetUserAccount(vmInput.RecipientAddr)
	if err != nil {
		return err
	}
	if check.IfNilReflect(contract) {
		return vmhost.ErrNilContract
	}

	codeMetadata := vmcommon.CodeMetadataFromBytes(contract.GetCodeMetadata())
	isUpgradeable := codeMetadata.Upgradeable
	callerAddress := vmInput.CallerAddr
	ownerAddress := contract.GetOwnerAddress()
	isCallerOwner := bytes.Equal(callerAddress, ownerAddress)

	if isUpgradeable && isCallerOwner {
		return nil
	}

	return vmhost.ErrUpgradeNotAllowed
}

// executeUpgrade upgrades a contract indirectly (from another contract). This
// function follows the convention of executeSmartContractCall().
func (host *vmHost) executeUpgrade(input *vmcommon.ContractCallInput) error {
	_, _, metering, output, runtime, _ := host.GetContexts()

	err := host.checkUpgradePermission(input)
	if err != nil {
		return err
	}

	code, codeMetadata, err := runtime.ExtractCodeUpgradeFromArgs()
	if err != nil {
		return vmhost.ErrInvalidUpgradeArguments
	}

	codeDeployInput := vmhost.CodeDeployInput{
		ContractCode:         code,
		ContractCodeMetadata: codeMetadata,
		ContractAddress:      input.RecipientAddr,
		CodeDeployerAddress:  input.CallerAddr,
	}

	err = metering.DeductInitialGasForDirectDeployment(codeDeployInput)
	if err != nil {
		output.SetReturnCode(vmcommon.OutOfGas)
		return err
	}

	runtime.MustVerifyNextContractCode()

	err = runtime.StartWasmerInstance(codeDeployInput.ContractCode, metering.GetGasForExecution(), true)
	if err != nil {
		log.Debug("performCodeDeployment/StartWasmerInstance", "err", err)
		return vmhost.ErrContractInvalid
	}

	err = host.callInitFunction()
	if err != nil {
		return err
	}

	output.DeployCode(codeDeployInput)
	if output.ReturnCode() != vmcommon.Ok {
		return vmhost.ErrReturnCodeNotOk
	}

	return nil
}

// executeSmartContractCall executes an indirect call to a smart contract,
// assuming there is an already-running Wasmer instance with another contract
// that has requested the indirect call. This method creates a new Wasmer
// instance and pushes the previous one onto the Runtime instance stack, but it
// will not pop the previous instance back - that remains the responsibility of
// the calling code. Also, this method does not restore the gas remaining after
// the indirect call, it does not push the states of any Host Context onto
// their respective stacks, nor does it pop any state stack. Handling the state
// stacks and the remaining gas are responsibilities of the calling code, which
// must push and pop as required, before and after calling this method, and
// handle the remaining gas. These principles also apply to indirect contract
// upgrading (via host.executeUpgrade(), which also does not pop the previous
// instance from the Runtime instance stack, nor does it restore the remaining
// gas).
func (host *vmHost) executeSmartContractCall(
	input *vmcommon.ContractCallInput,
	metering vmhost.MeteringContext,
	runtime vmhost.RuntimeContext,
	output vmhost.OutputContext,
	withInitialGasDeduct bool,
) error {
	if host.isInitFunctionBeingCalled() && !input.AllowInitFunction {
		return vmhost.ErrInitFuncCalledInRun
	}

	// Use all gas initially, on the Wasmer instance of the caller. In case of
	// successful execution, the unused gas will be restored.
	metering.UseGas(input.GasProvided)

	isUpgrade := input.Function == vmhost.UpgradeFunctionName
	if isUpgrade {
		return host.executeUpgrade(input)
	}

	contract, err := runtime.GetSCCode()
	if err != nil {
		return err
	}

	if withInitialGasDeduct {
		err = metering.DeductInitialGasForExecution(contract)
		if err != nil {
			return err
		}
	}

	// Replace the current Wasmer instance of the Runtime with a new one; this
	// assumes that the instance was preserved on the Runtime instance stack
	// before calling executeSmartContractCall().
	err = runtime.StartWasmerInstance(contract, metering.GetGasForExecution(), false)
	if err != nil {
		return err
	}

	err = host.callSCMethodIndirect()
	if err != nil {
		return err
	}

	if output.ReturnCode() != vmcommon.Ok {
		return vmhost.ErrReturnCodeNotOk
	}

	return nil
}

func (host *vmHost) execute(input *vmcommon.ContractCallInput) (uint64, error) {
	_, _, metering, output, runtime, storage := host.GetContexts()

	if host.isBuiltinFunctionBeingCalled() {
		newVMInput, gasUsedBeforeReset, err := host.callBuiltinFunction(input)
		if err != nil {
			return gasUsedBeforeReset, err
		}

		if newVMInput != nil {
			runtime.InitStateFromContractCallInput(newVMInput)
			metering.InitStateFromContractCallInput(&newVMInput.VMInput)
			storage.SetAddress(runtime.GetSCAddress())
			err = host.executeSmartContractCall(newVMInput, metering, runtime, output, false)
			if err != nil {
				host.RevertDCDTTransfer(input)
			}

			return gasUsedBeforeReset, err
		}

		return gasUsedBeforeReset, nil
	}

	return 0, host.executeSmartContractCall(input, metering, runtime, output, true)
}

func (host *vmHost) computeGasUsedBefore() {
	_, _, metering, output, _, _ := host.GetContexts()
	gasUsed, _ := output.GetCurrentTotalUsedGas()
	metering.SetTotalUsedGas(gasUsed)
}

func (host *vmHost) callSCMethodIndirect() error {
	function, err := host.Runtime().GetFunctionToCall()
	if err != nil {
		if errors.Is(err, vmhost.ErrNilCallbackFunction) {
			return nil
		}
		return err
	}

	_, err = function()
	if err != nil {
		err = host.handleBreakpointIfAny(err)
	}

	return err
}

// RevertDCDTTransfer calls the DCDT/DCDTNFT transfer with reverted arguments
func (host *vmHost) RevertDCDTTransfer(input *vmcommon.ContractCallInput) {
	isDCDTTransfer := input.Function == core.BuiltInFunctionDCDTTransfer || input.Function == core.BuiltInFunctionDCDTNFTTransfer
	if !isDCDTTransfer {
		return
	}
	if input.CallType == vm.AsynchronousCallBack {
		return
	}
	numArgsForTransfer := core.MinLenArgumentsDCDTTransfer
	if input.Function == core.BuiltInFunctionDCDTNFTTransfer {
		numArgsForTransfer = core.MinLenArgumentsDCDTNFTTransfer
	}
	if len(input.Arguments) < numArgsForTransfer {
		return
	}

	revertInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:     input.RecipientAddr,
			Arguments:      make([][]byte, numArgsForTransfer),
			CallValue:      big.NewInt(0),
			CallType:       vm.AsynchronousCallBack,
			GasPrice:       input.GasPrice,
			GasProvided:    host.meteringContext.BlockGasLimit(),
			GasLocked:      0,
			OriginalTxHash: input.OriginalTxHash,
			CurrentTxHash:  input.CurrentTxHash,
		},
		RecipientAddr:     input.CallerAddr,
		Function:          input.Function,
		AllowInitFunction: false,
	}
	copy(revertInput.Arguments, input.Arguments)
	if input.Function == core.BuiltInFunctionDCDTNFTTransfer {
		actualRecipient := input.CallerAddr
		actualSender := input.Arguments[numArgsForTransfer-1]

		revertInput.RecipientAddr = actualSender
		revertInput.CallerAddr = actualSender
		revertInput.Arguments[numArgsForTransfer-1] = actualRecipient
	}

	vmOutput, err := host.blockChainHook.ProcessBuiltInFunction(revertInput)
	if err != nil {
		log.Error("RevertDCDTTransfer failed", "error", err)
		host.meteringContext.UseGas(host.meteringContext.GasLeft())
		host.runtimeContext.FailExecution(err)
		return
	}
	if vmOutput != nil && vmOutput.ReturnCode != vmcommon.Ok {
		log.Error("RevertDCDTTransfer failed", "returnCode", vmOutput.ReturnCode, "returnMessage", vmOutput.ReturnMessage)
		host.meteringContext.UseGas(host.meteringContext.GasLeft())
		host.runtimeContext.FailExecution(errors.New(vmOutput.ReturnMessage))
	}
}

// ExecuteDCDTTransfer calls the process built in function with the given transfer for DCDT/DCDTNFT if nonce > 0
// there are no NFTs with nonce == 0
func (host *vmHost) ExecuteDCDTTransfer(destination []byte, sender []byte, tokenIdentifier []byte, nonce uint64, value *big.Int, callType vm.CallType, isRevert bool) (*vmcommon.VMOutput, uint64, error) {
	_, _, metering, _, runtime, _ := host.GetContexts()

	dcdtTransferInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:  sender,
			Arguments:   make([][]byte, 0),
			CallValue:   big.NewInt(0),
			CallType:    callType,
			GasPrice:    runtime.GetVMInput().GasPrice,
			GasProvided: metering.GasLeft(),
			GasLocked:   0,
		},
		RecipientAddr:     destination,
		Function:          core.BuiltInFunctionDCDTTransfer,
		AllowInitFunction: false,
	}

	if nonce > 0 {
		dcdtTransferInput.Function = core.BuiltInFunctionDCDTNFTTransfer
		dcdtTransferInput.RecipientAddr = dcdtTransferInput.CallerAddr
		nonceAsBytes := big.NewInt(0).SetUint64(nonce).Bytes()
		dcdtTransferInput.Arguments = append(dcdtTransferInput.Arguments, tokenIdentifier, nonceAsBytes, value.Bytes(), destination)
	} else {
		dcdtTransferInput.Arguments = append(dcdtTransferInput.Arguments, tokenIdentifier, value.Bytes())
	}

	if isRevert {
		dcdtTransferInput.GasProvided = metering.BlockGasLimit()
	}

	vmOutput, err := host.blockChainHook.ProcessBuiltInFunction(dcdtTransferInput)
	log.Trace("DCDT transfer", "sender", sender, "dest", destination)
	log.Trace("DCDT transfer", "token", tokenIdentifier, "value", value)
	if err != nil {
		log.Trace("DCDT transfer", "error", err)
		return vmOutput, dcdtTransferInput.GasProvided, err
	}
	if vmOutput.ReturnCode != vmcommon.Ok {
		log.Trace("DCDT transfer", "error", err, "retcode", vmOutput.ReturnCode, "message", vmOutput.ReturnMessage)
		return vmOutput, dcdtTransferInput.GasProvided, vmhost.ErrExecutionFailed
	}

	gasConsumed, _ := math.SubUint64(dcdtTransferInput.GasProvided, vmOutput.GasRemaining)
	for _, outAcc := range vmOutput.OutputAccounts {
		for _, transfer := range outAcc.OutputTransfers {
			gasConsumed, _ = math.SubUint64(gasConsumed, transfer.GasLimit)
		}
	}
	if callType != vm.AsynchronousCallBack && !isRevert {
		if metering.GasLeft() < gasConsumed {
			log.Trace("DCDT transfer", "error", vmhost.ErrNotEnoughGas)
			return vmOutput, dcdtTransferInput.GasProvided, vmhost.ErrNotEnoughGas
		}
		metering.UseGas(gasConsumed)
	}

	return vmOutput, gasConsumed, nil
}

func (host *vmHost) callBuiltinFunction(input *vmcommon.ContractCallInput) (*vmcommon.ContractCallInput, uint64, error) {
	_, _, metering, output, runtime, _ := host.GetContexts()

	gasConsumedForExecution := host.computeGasUsedInExecutionBeforeReset(input)
	runtime.SetPointsUsed(0)
	vmOutput, err := host.blockChainHook.ProcessBuiltInFunction(input)
	if err != nil {
		metering.UseGas(input.GasProvided)
		return nil, gasConsumedForExecution, err
	}

	gasConsumed, _ := math.SubUint64(input.GasProvided, vmOutput.GasRemaining)
	for _, outAcc := range vmOutput.OutputAccounts {
		for _, outTransfer := range outAcc.OutputTransfers {
			if outTransfer.GasLimit > 0 || outTransfer.GasLocked > 0 {
				gasForwarded := math.AddUint64(outTransfer.GasLocked, outTransfer.GasLimit)
				gasConsumed = math.AddUint64(gasConsumed, outTransfer.GasLocked)

				if input.CallType != vm.AsynchronousCallBack {
					metering.ForwardGas(runtime.GetSCAddress(), nil, gasForwarded)
				} else {
					gasConsumed, _ = math.SubUint64(gasConsumed, outTransfer.GasLimit)
				}
			}
		}
	}

	if vmOutput.GasRemaining < input.GasProvided {
		metering.UseGas(gasConsumed)
	}

	newVMInput, err := host.isSCExecutionAfterBuiltInFunc(input, vmOutput)
	if err != nil {
		return nil, gasConsumedForExecution, err
	}

	if newVMInput != nil {
		for _, outAcc := range vmOutput.OutputAccounts {
			outAcc.OutputTransfers = make([]vmcommon.OutputTransfer, 0)
		}
	}

	host.addDCDTTransferToVMOutputSCIntraShardCall(input, vmOutput)
	output.AddToActiveState(vmOutput)

	return newVMInput, gasConsumedForExecution, nil
}

// add output transfer of dcdt transfer when sc calling another sc intra shard to log the transfer information
func (host *vmHost) addDCDTTransferToVMOutputSCIntraShardCall(
	input *vmcommon.ContractCallInput,
	output *vmcommon.VMOutput,
) {
	if output.ReturnCode != vmcommon.Ok {
		return
	}
	if !host.AreInSameShard(input.RecipientAddr, input.CallerAddr) {
		return
	}
	isDCDTTransfer := input.Function == core.BuiltInFunctionDCDTTransfer || input.Function == core.BuiltInFunctionDCDTNFTTransfer
	if !isDCDTTransfer {
		return
	}

	recipientAddr := input.RecipientAddr
	if input.Function == core.BuiltInFunctionDCDTNFTTransfer {
		if len(input.Arguments) != 4 {
			return
		}
		recipientAddr = input.Arguments[3]
	}
	addOutputTransferToVMOutput(input.Function, input.Arguments, input.CallerAddr, recipientAddr, input.CallType, output)
}

func addOutputTransferToVMOutput(
	function string,
	arguments [][]byte,
	sender []byte,
	recipient []byte,
	callType vm.CallType,
	vmOutput *vmcommon.VMOutput,
) {
	dcdtTransferTxData := function
	for _, arg := range arguments {
		dcdtTransferTxData += "@" + hex.EncodeToString(arg)
	}
	outTransfer := vmcommon.OutputTransfer{
		Value:         big.NewInt(0),
		Data:          []byte(dcdtTransferTxData),
		CallType:      callType,
		SenderAddress: sender,
	}

	if len(vmOutput.OutputAccounts) == 0 {
		vmOutput.OutputAccounts = make(map[string]*vmcommon.OutputAccount)
	}
	outAcc, ok := vmOutput.OutputAccounts[string(recipient)]
	if !ok {
		outAcc = &vmcommon.OutputAccount{
			Address:         recipient,
			OutputTransfers: make([]vmcommon.OutputTransfer, 0),
		}
	}
	outAcc.OutputTransfers = append(outAcc.OutputTransfers, outTransfer)
	vmOutput.OutputAccounts[string(recipient)] = outAcc
}

func (host *vmHost) checkFinalGasAfterExit() error {
	if !host.IsVMV2Enabled() {
		return nil
	}

	totalUsedPoints := host.Runtime().GetPointsUsed()
	if totalUsedPoints > host.Metering().GetGasForExecution() {
		return vmhost.ErrNotEnoughGas
	}

	return nil
}

func (host *vmHost) callInitFunction() error {
	runtime := host.Runtime()
	init := runtime.GetInitFunction()
	if init == nil {
		return nil
	}

	_, err := init()
	if err != nil {
		err = host.handleBreakpointIfAny(err)
	}

	if err == nil {
		err = host.checkFinalGasAfterExit()
	}

	return err
}

func (host *vmHost) callSCMethod() error {
	runtime := host.Runtime()

	log.Trace("call SC method")

	// TODO host.verifyAllowedFunctionCall() performs some checks, but then the
	// function itself is changed by host.getFunctionByCallType(). Order must be
	// reversed, and `getFunctionByCallType()` must be decomposed into smaller functions.

	err := host.verifyAllowedFunctionCall()
	if err != nil {
		log.Trace("call SC method failed", "error", err)
		return err
	}

	callType := runtime.GetVMInput().CallType
	function, err := host.getFunctionByCallType(callType)
	if err != nil {
		if callType == vm.AsynchronousCallBack && errors.Is(err, vmhost.ErrNilCallbackFunction) {
			err = host.processCallbackStack()
			if err != nil {
				log.Trace("call SC method failed", "error", err)
			}

			return err
		}
		log.Trace("call SC method failed", "error", err)
		return err
	}

	_, err = function()
	if err != nil {
		err = host.handleBreakpointIfAny(err)
	}
	if err == nil {
		err = host.checkFinalGasAfterExit()
	}
	if err != nil {
		log.Trace("call SC method failed", "error", err)
		return err
	}

	switch callType {
	case vm.AsynchronousCall:
		pendingMap, paiErr := host.processAsyncInfo(runtime.GetAsyncContextInfo())
		if paiErr != nil {
			log.Trace("call SC method failed", "error", paiErr)
			return paiErr
		}
		if len(pendingMap.AsyncContextMap) == 0 {
			err = host.sendCallbackToCurrentCaller()
		}
	case vm.AsynchronousCallBack:
		err = host.processCallbackStack()
	default:
		_, err = host.processAsyncInfo(runtime.GetAsyncContextInfo())
	}

	if err != nil {
		log.Trace("call SC method failed", "error", err)
	}

	return err
}

func (host *vmHost) verifyAllowedFunctionCall() error {
	runtime := host.Runtime()
	functionName := runtime.Function()

	isInit := functionName == vmhost.InitFunctionName || functionName == vmhost.InitFunctionNameEth
	if isInit {
		return vmhost.ErrInitFuncCalledInRun
	}

	isCallBack := functionName == vmhost.CallbackFunctionName
	isInAsyncCallBack := runtime.GetVMInput().CallType == vm.AsynchronousCallBack
	if isCallBack && !isInAsyncCallBack {
		return vmhost.ErrCallBackFuncCalledInRun
	}

	return nil
}

func (host *vmHost) isSCExecutionAfterBuiltInFunc(
	vmInput *vmcommon.ContractCallInput,
	vmOutput *vmcommon.VMOutput,
) (*vmcommon.ContractCallInput, error) {
	if vmOutput.ReturnCode != vmcommon.Ok {
		return nil, nil
	}
	recipient := vmInput.RecipientAddr
	if vmInput.Function == core.BuiltInFunctionDCDTNFTTransfer && bytes.Equal(vmInput.CallerAddr, vmInput.RecipientAddr) {
		recipient = vmInput.Arguments[3]
	}
	if !host.AreInSameShard(vmInput.CallerAddr, recipient) {
		return nil, nil
	}
	if !host.Blockchain().IsSmartContract(recipient) {
		return nil, nil
	}

	outAcc, ok := vmOutput.OutputAccounts[string(recipient)]
	if !ok {
		return nil, nil
	}
	if len(outAcc.OutputTransfers) != 1 {
		return nil, nil
	}

	callType := vmInput.CallType
	scCallOutTransfer := outAcc.OutputTransfers[0]

	argParser := parsers.NewCallArgsParser()
	function, arguments, err := argParser.ParseData(string(scCallOutTransfer.Data))
	if err != nil {
		return nil, err
	}

	newVMInput := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallerAddr:     vmInput.CallerAddr,
			Arguments:      arguments,
			CallValue:      big.NewInt(0),
			CallType:       callType,
			GasPrice:       vmInput.GasPrice,
			GasProvided:    scCallOutTransfer.GasLimit,
			GasLocked:      scCallOutTransfer.GasLocked,
			OriginalTxHash: vmInput.OriginalTxHash,
			CurrentTxHash:  vmInput.CurrentTxHash,
		},
		RecipientAddr:     recipient,
		Function:          function,
		AllowInitFunction: false,
	}

	fillWithDCDTValue(vmInput, newVMInput)

	return newVMInput, nil
}

func fillWithDCDTValue(fullVMInput *vmcommon.ContractCallInput, newVMInput *vmcommon.ContractCallInput) {
	isDCDTTransfer := fullVMInput.Function == core.BuiltInFunctionDCDTTransfer || fullVMInput.Function == core.BuiltInFunctionDCDTNFTTransfer
	if !isDCDTTransfer {
		return
	}

	dcdtTransfer := &vmcommon.DCDTTransfer{}

	dcdtTransfer.DCDTTokenName = fullVMInput.Arguments[0]
	dcdtTransfer.DCDTValue = big.NewInt(0).SetBytes(fullVMInput.Arguments[1])

	if fullVMInput.Function == core.BuiltInFunctionDCDTNFTTransfer {
		dcdtTransfer.DCDTTokenNonce = big.NewInt(0).SetBytes(fullVMInput.Arguments[1]).Uint64()
		dcdtTransfer.DCDTValue = big.NewInt(0).SetBytes(fullVMInput.Arguments[2])
		dcdtTransfer.DCDTTokenType = uint32(core.NonFungible)
	}

	newVMInput.DCDTTransfers = make([]*vmcommon.DCDTTransfer, 1)
	newVMInput.DCDTTransfers[0] = dcdtTransfer
}
