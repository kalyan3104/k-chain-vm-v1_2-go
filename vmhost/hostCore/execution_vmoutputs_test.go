package hostCore

import (
	"fmt"
	"math/big"

	"github.com/kalyan3104/k-chain-core-go/data/vm"
	vmcommon "github.com/kalyan3104/k-chain-vm-common-go"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/vmhost"
)

var parentKeyA = []byte("parentKeyA......................")
var parentKeyB = []byte("parentKeyB......................")
var childKey = []byte("childKey........................")
var parentDataA = []byte("parentDataA")
var parentDataB = []byte("parentDataB")
var childData = []byte("childData")
var parentFinishA = []byte("parentFinishA")
var parentFinishB = []byte("parentFinishB")
var childFinish = []byte("childFinish")
var parentTransferReceiver = []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x0f\x0fparentTransferReceiver")
var childTransferReceiver = []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x0f\x0fchildTransferReceiver.")
var parentTransferValue = int64(42)
var parentTransferData = []byte("parentTransferData")

var recursiveIterationCounterKey = []byte("recursiveIterationCounter.......")
var recursiveIterationBigCounterKey = []byte("recursiveIterationBigCounter....")

var gasProvided = uint64(1000000)

var parentCompilationCostSameCtx uint64
var childCompilationCostSameCtx uint64

var parentCompilationCostDestCtx uint64
var childCompilationCostDestCtx uint64

var vaultAddress = []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x0f\x0fvaultAddress..........")
var thirdPartyAddress = []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x0f\x0fthirdPartyAddress.....")

func init() {
	parentCompilationCostSameCtx = uint64(len(GetTestSCCode("exec-same-ctx-parent", "../../")))
	childCompilationCostSameCtx = uint64(len(GetTestSCCode("exec-same-ctx-child", "../../")))

	parentCompilationCostDestCtx = uint64(len(GetTestSCCode("exec-dest-ctx-parent", "../../")))
	childCompilationCostDestCtx = uint64(len(GetTestSCCode("exec-dest-ctx-child", "../../")))
}

func expectedVMOutputSameCtxPrepare(_ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()
	vmOutput.ReturnCode = vmcommon.Ok

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-parentTransferValue,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 3405

	_ = AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentTransferReceiver,
		parentTransferValue,
		parentTransferData,
	)

	SetStorageUpdate(parentAccount, parentKeyA, parentDataA)
	SetStorageUpdate(parentAccount, parentKeyB, parentDataB)

	AddFinishData(vmOutput, parentFinishA)
	AddFinishData(vmOutput, parentFinishB)
	AddFinishData(vmOutput, []byte("succ"))

	expectedExecutionCost := uint64(138)
	gas := gasProvided
	gas -= parentCompilationCostSameCtx
	gas -= expectedExecutionCost
	vmOutput.GasRemaining = gas

	return vmOutput
}

func expectedVMOutputSameCtxWrongContractCalled(code []byte) *vmcommon.VMOutput {
	vmOutput := expectedVMOutputSameCtxPrepare(code)

	AddFinishData(vmOutput, []byte("fail"))

	executionCostBeforeExecuteAPI := uint64(156)
	executeAPICost := uint64(39)
	gasLostOnFailure := uint64(50000)
	finalCost := uint64(44)
	gas := gasProvided
	gas -= parentCompilationCostSameCtx
	gas -= executionCostBeforeExecuteAPI
	gas -= executeAPICost
	gas -= gasLostOnFailure
	gas -= finalCost
	vmOutput.GasRemaining = gas

	return vmOutput
}

func expectedVMOutputSameCtxOutOfGas(_ []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	vmOutput.ReturnCode = vmcommon.Ok

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)

	SetStorageUpdate(parentAccount, parentKeyA, parentDataA)
	SetStorageUpdate(parentAccount, parentKeyB, parentDataB)
	AddFinishData(vmOutput, parentFinishA)

	AddFinishData(vmOutput, []byte("fail"))

	executionCostBeforeExecuteAPI := uint64(90)
	executeAPICost := uint64(1)
	gasLostOnFailure := uint64(3500)
	finalCost := uint64(54)
	gas := gasProvided
	gas -= parentCompilationCostSameCtx
	gas -= executionCostBeforeExecuteAPI
	gas -= executeAPICost
	gas -= gasLostOnFailure
	gas -= finalCost
	vmOutput.GasRemaining = gas

	return vmOutput
}

func expectedVMOutputSameCtxSimple(_ []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	AddFinishData(vmOutput, []byte("child"))
	AddFinishData(vmOutput, []byte{})
	for i := 1; i < 100; i++ {
		AddFinishData(vmOutput, []byte{byte(i)})
	}
	AddFinishData(vmOutput, []byte{})
	AddFinishData(vmOutput, []byte("child"))
	AddFinishData(vmOutput, []byte{})
	for i := 1; i < 100; i++ {
		AddFinishData(vmOutput, []byte{byte(i)})
	}
	AddFinishData(vmOutput, []byte{})
	AddFinishData(vmOutput, []byte("parent"))

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-198,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 521

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		198,
		nil,
	)
	childAccount.GasUsed = 3435 // TODO: double this when fixed

	executionCost := parentAccount.GasUsed + 2*childAccount.GasUsed
	vmOutput.GasRemaining = gasProvided - executionCost

	return vmOutput
}

func expectedVMOutputSameCtxSuccessfulChildCall(parentCode []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := expectedVMOutputSameCtxPrepare(parentCode)

	parentAccount := vmOutput.OutputAccounts[string(parentAddress)]
	parentAccount.BalanceDelta = big.NewInt(-141)
	parentAccount.GasUsed = 3612

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		3,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)

	executeAPICost := uint64(39)
	childExecutionCost := uint64(437)
	childAccount.GasUsed = childCompilationCostSameCtx + childExecutionCost

	_ = AddNewOutputAccount(
		vmOutput,
		childAddress,
		childTransferReceiver,
		96,
		[]byte("qwerty"),
	)

	// The child SC has stored data on the parent's storage
	SetStorageUpdate(parentAccount, childKey, childData)

	// The called child SC will output some arbitrary data, but also data that it
	// has read from the storage of the parent.
	AddFinishData(vmOutput, childFinish)
	AddFinishData(vmOutput, parentDataA)
	for _, c := range parentDataA {
		AddFinishData(vmOutput, []byte{c})
	}
	AddFinishData(vmOutput, parentDataB)
	for _, c := range parentDataB {
		AddFinishData(vmOutput, []byte{c})
	}
	AddFinishData(vmOutput, []byte("child ok"))
	AddFinishData(vmOutput, []byte("succ"))
	AddFinishData(vmOutput, []byte("succ"))

	parentGasBeforeExecuteAPI := uint64(172)
	finalCost := uint64(134)
	gas := gasProvided
	gas -= parentCompilationCostSameCtx
	gas -= parentGasBeforeExecuteAPI
	gas -= executeAPICost
	gas -= childCompilationCostSameCtx
	gas -= childExecutionCost
	gas -= finalCost
	vmOutput.GasRemaining = gas

	return vmOutput
}

func expectedVMOutputSameCtxSuccessfulChildCallBigInts(_ []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()
	vmOutput.ReturnCode = vmcommon.Ok

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-99,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 3461

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		99,
		nil,
	)
	childExecutionCost := uint64(108)
	childAccount.GasUsed = childCompilationCostSameCtx + childExecutionCost

	// The child SC will output "child ok" if it could read some expected Big
	// Ints directly from the parent's context.
	AddFinishData(vmOutput, []byte("child ok"))
	AddFinishData(vmOutput, []byte("succ"))
	AddFinishData(vmOutput, []byte("succ"))

	parentGasBeforeExecuteAPI := uint64(114)
	executeAPICost := uint64(13)
	finalCost := uint64(67)
	gas := gasProvided
	gas -= parentCompilationCostSameCtx
	gas -= parentGasBeforeExecuteAPI
	gas -= executeAPICost
	gas -= childCompilationCostSameCtx
	gas -= childExecutionCost
	gas -= finalCost
	vmOutput.GasRemaining = gas
	return vmOutput
}

func expectedVMOutputSameCtxRecursiveDirect(_ []byte, recursiveCalls int) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	account := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	account.Balance = big.NewInt(1000)
	account.BalanceDelta = big.NewInt(0).Sub(big.NewInt(1), big.NewInt(1))
	account.GasUsed = 25424

	for i := recursiveCalls; i >= 0; i-- {
		finishString := fmt.Sprintf("Rfinish%03d", i)
		AddFinishData(vmOutput, []byte(finishString))
	}

	for i := recursiveCalls - 1; i >= 0; i-- {
		AddFinishData(vmOutput, []byte("succ"))
	}

	for i := 0; i <= recursiveCalls; i++ {
		key := fmt.Sprintf("Rkey%03d.........................", i)
		value := fmt.Sprintf("Rvalue%03d", i)
		SetStorageUpdateStrings(account, key, value)
	}

	SetStorageUpdate(account, recursiveIterationCounterKey, []byte{byte(recursiveCalls + 1)})
	SetStorageUpdate(account, recursiveIterationBigCounterKey, big.NewInt(int64(recursiveCalls+1)).Bytes())

	return vmOutput
}

func expectedVMOutputSameCtxRecursiveDirectErrMaxInstances(_ []byte, recursiveCalls int) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	account := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)

	finishString := fmt.Sprintf("Rfinish%03d", recursiveCalls)
	AddFinishData(vmOutput, []byte(finishString))

	key := fmt.Sprintf("Rkey%03d.........................", recursiveCalls)
	value := fmt.Sprintf("Rvalue%03d", recursiveCalls)
	SetStorageUpdateStrings(account, key, value)

	AddFinishData(vmOutput, []byte("fail"))
	SetStorageUpdate(account, recursiveIterationCounterKey, []byte{byte(1)})

	return vmOutput
}

func expectedVMOutputSameCtxRecursiveMutualMethods(_ []byte, recursiveCalls int) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	account := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	account.Balance = big.NewInt(1000)
	account.BalanceDelta = big.NewInt(0).Sub(big.NewInt(1), big.NewInt(1))
	account.GasUsed = 29593

	SetStorageUpdate(account, recursiveIterationCounterKey, []byte{byte(recursiveCalls + 1)})
	SetStorageUpdate(account, recursiveIterationBigCounterKey, big.NewInt(int64(recursiveCalls+1)).Bytes())

	AddFinishData(vmOutput, []byte("start recursive mutual calls"))

	for i := 0; i <= recursiveCalls; i++ {
		var finishData string
		var key string
		var value string
		iteration := recursiveCalls - i
		if i%2 == 0 {
			finishData = fmt.Sprintf("Afinish%03d", iteration)
			key = fmt.Sprintf("Akey%03d.........................", iteration)
			value = fmt.Sprintf("Avalue%03d", iteration)
		} else {
			finishData = fmt.Sprintf("Bfinish%03d", iteration)
			key = fmt.Sprintf("Bkey%03d.........................", iteration)
			value = fmt.Sprintf("Bvalue%03d", iteration)
		}
		SetStorageUpdateStrings(account, key, value)
		AddFinishData(vmOutput, []byte(finishData))
	}

	for i := recursiveCalls; i >= 0; i-- {
		AddFinishData(vmOutput, []byte("succ"))
	}

	AddFinishData(vmOutput, []byte("end recursive mutual calls"))

	return vmOutput
}

func expectedVMOutputSameCtxRecursiveMutualSCs(_ []byte, _ []byte, recursiveCalls int) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 5426

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		0,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)
	childAccount.GasUsed = 3652

	if recursiveCalls%2 == 1 {
		parentAccount.BalanceDelta = big.NewInt(-5)
		childAccount.BalanceDelta = big.NewInt(5)
	} else {
		parentAccount.BalanceDelta = big.NewInt(0).Sub(big.NewInt(1), big.NewInt(1))
		childAccount.BalanceDelta = big.NewInt(0).Sub(big.NewInt(1), big.NewInt(1))
	}

	for i := 0; i <= recursiveCalls; i++ {
		var finishData string
		var key string
		var value string
		iteration := recursiveCalls - i
		if i%2 == 0 {
			finishData = fmt.Sprintf("Pfinish%03d", iteration)
			key = fmt.Sprintf("Pkey%03d.........................", iteration)
			value = fmt.Sprintf("Pvalue%03d", iteration)
		} else {
			finishData = fmt.Sprintf("Cfinish%03d", iteration)
			key = fmt.Sprintf("Ckey%03d.........................", iteration)
			value = fmt.Sprintf("Cvalue%03d", iteration)
		}
		SetStorageUpdateStrings(parentAccount, key, value)
		AddFinishData(vmOutput, []byte(finishData))
	}

	for i := recursiveCalls - 1; i >= 0; i-- {
		AddFinishData(vmOutput, []byte("succ"))
	}

	SetStorageUpdate(parentAccount, recursiveIterationCounterKey, []byte{byte(recursiveCalls + 1)})
	SetStorageUpdate(parentAccount, recursiveIterationBigCounterKey, big.NewInt(int64(recursiveCalls+1)).Bytes())

	return vmOutput
}

func expectedVMOutputDestCtxBuiltinClaim(input *vmcommon.ContractCallInput, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	account := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		42,
		nil,
	)
	account.Balance = big.NewInt(1000)
	account.GasUsed = 1988

	AddFinishData(vmOutput, []byte("succ"))

	vmOutput.GasRemaining = input.GasProvided
	vmOutput.GasRemaining -= account.GasUsed

	return vmOutput
}

func expectedVMOutputDestCtxBuiltinDoSomething(input *vmcommon.ContractCallInput, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	account := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	account.Balance = big.NewInt(1000)
	account.BalanceDelta = big.NewInt(0)
	account.GasUsed = 1992

	AddFinishData(vmOutput, []byte("succ"))

	vmOutput.GasRemaining = input.GasProvided
	vmOutput.GasRemaining -= account.GasUsed

	return vmOutput
}

func expectedVMOutputDestCtxBuiltinNonexistent(_ *vmcommon.ContractCallInput, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutputError()
	vmOutput.ReturnMessage = vmhost.ErrFuncNotFound.Error()

	return vmOutput
}

func expectedVMOutputDestCtxBuiltinFail(_ *vmcommon.ContractCallInput, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutputError()
	vmOutput.ReturnMessage = "whatdidyoudo"

	return vmOutput
}

func expectedVMOutputDestCtxPrepare(_ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()
	vmOutput.ReturnCode = vmcommon.Ok

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-parentTransferValue,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 4309

	_ = AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentTransferReceiver,
		parentTransferValue,
		parentTransferData,
	)

	SetStorageUpdate(parentAccount, parentKeyA, parentDataA)
	SetStorageUpdate(parentAccount, parentKeyB, parentDataB)

	AddFinishData(vmOutput, parentFinishA)
	AddFinishData(vmOutput, parentFinishB)
	AddFinishData(vmOutput, []byte("succ"))

	expectedExecutionCost := uint64(138)
	gas := gasProvided
	gas -= parentCompilationCostDestCtx
	gas -= expectedExecutionCost
	vmOutput.GasRemaining = gas

	return vmOutput
}

func expectedVMOutputDestCtxWrongContractCalled(parentCode []byte) *vmcommon.VMOutput {
	vmOutput := expectedVMOutputSameCtxPrepare(parentCode)

	parentAccount := vmOutput.OutputAccounts[string(parentAddress)]
	parentAccount.BalanceDelta = big.NewInt(-42)

	AddFinishData(vmOutput, []byte("fail"))

	executionCostBeforeExecuteAPI := uint64(156)
	executeAPICost := uint64(42)
	gasLostOnFailure := uint64(10000)
	finalCost := uint64(44)
	gas := gasProvided
	gas -= parentCompilationCostDestCtx
	gas -= executionCostBeforeExecuteAPI
	gas -= executeAPICost
	gas -= gasLostOnFailure
	gas -= finalCost
	vmOutput.GasRemaining = gas

	return vmOutput
}

func expectedVMOutputDestCtxOutOfGas(_ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	vmOutput.ReturnCode = vmcommon.Ok

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)

	SetStorageUpdate(parentAccount, parentKeyA, parentDataA)
	SetStorageUpdate(parentAccount, parentKeyB, parentDataB)
	AddFinishData(vmOutput, parentFinishA)

	AddFinishData(vmOutput, []byte("fail"))

	executionCostBeforeExecuteAPI := uint64(90)
	executeAPICost := uint64(1)
	gasLostOnFailure := uint64(3500)
	finalCost := uint64(54)
	gas := gasProvided
	gas -= parentCompilationCostDestCtx
	gas -= executionCostBeforeExecuteAPI
	gas -= executeAPICost
	gas -= gasLostOnFailure
	gas -= finalCost
	vmOutput.GasRemaining = gas

	return vmOutput
}

func expectedVMOutputDestCtxSuccessfulChildCall(parentCode []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := expectedVMOutputSameCtxPrepare(parentCode)

	parentAccount := vmOutput.OutputAccounts[string(parentAddress)]
	parentAccount.BalanceDelta = big.NewInt(-141)
	parentAccount.GasUsed = 4444

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		99-12,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)
	childAccount.GasUsed = 2256

	_ = AddNewOutputAccount(
		vmOutput,
		childAddress,
		childTransferReceiver,
		12,
		[]byte("Second sentence."),
	)

	SetStorageUpdate(parentAccount, childKey, nil)
	SetStorageUpdate(childAccount, childKey, childData)

	AddFinishData(vmOutput, childFinish)
	AddFinishData(vmOutput, []byte("succ"))
	AddFinishData(vmOutput, []byte("succ"))

	parentGasBeforeExecuteAPI := uint64(168)
	executeAPICost := uint64(42)
	childExecutionCost := uint64(91)
	finalCost := uint64(65)
	gas := gasProvided
	gas -= parentCompilationCostDestCtx
	gas -= parentGasBeforeExecuteAPI
	gas -= executeAPICost
	gas -= childCompilationCostDestCtx
	gas -= childExecutionCost
	gas -= finalCost
	vmOutput.GasRemaining = gas
	return vmOutput
}

func expectedVMOutputDestCtxSuccessfulChildCallChildReturns(parentCode []byte, _ []byte) *vmcommon.VMOutput {

	vmOutput := expectedVMOutputSameCtxPrepare(parentCode)

	parentAccount := vmOutput.OutputAccounts[string(parentAddress)]
	parentAccount.BalanceDelta = big.NewInt(-141)
	parentAccount.GasUsed = 4652

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		99-12,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)
	childAccount.GasUsed = 2256

	_ = AddNewOutputAccount(
		vmOutput,
		childAddress,
		childTransferReceiver,
		12,
		[]byte("Second sentence."),
	)

	SetStorageUpdate(childAccount, childKey, childData)

	AddFinishData(vmOutput, childFinish)
	AddFinishData(vmOutput, []byte("succ"))

	parentGasBeforeExecuteAPI := uint64(168)
	executeAPICost := uint64(42)
	childExecutionCost := uint64(91)
	parentGasAfterExecuteAPI := uint64(273)
	gas := gasProvided
	gas -= parentCompilationCostDestCtx
	gas -= parentGasBeforeExecuteAPI
	gas -= executeAPICost
	gas -= childCompilationCostDestCtx
	gas -= childExecutionCost
	gas -= parentGasAfterExecuteAPI
	vmOutput.GasRemaining = gas

	return vmOutput
}

func expectedVMOutputDestCtxSuccessfulChildCallBigInts(_ []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()
	vmOutput.ReturnCode = vmcommon.Ok

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-99,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 4366

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		99,
		nil,
	)
	childAccount.GasUsed = 2265

	// The child SC will output "child ok" if it could NOT read the Big Ints from
	// the parent's context.
	AddFinishData(vmOutput, []byte("child ok"))
	AddFinishData(vmOutput, []byte("succ"))
	AddFinishData(vmOutput, []byte("succ"))

	parentGasBeforeExecuteAPI := uint64(115)
	executeAPICost := uint64(13)
	childExecutionCost := uint64(101)
	finalCost := uint64(68)
	gas := gasProvided
	gas -= parentCompilationCostDestCtx
	gas -= parentGasBeforeExecuteAPI
	gas -= executeAPICost
	gas -= childCompilationCostDestCtx
	gas -= childExecutionCost
	gas -= finalCost
	vmOutput.GasRemaining = gas
	return vmOutput
}

func expectedVMOutputDestCtxRecursiveDirect(_ []byte, recursiveCalls int) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	account := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	account.Balance = big.NewInt(1000)
	account.BalanceDelta = big.NewInt(0).Sub(big.NewInt(1), big.NewInt(1))
	account.GasUsed = 29670

	for i := recursiveCalls; i >= 0; i-- {
		finishString := fmt.Sprintf("Rfinish%03d", i)
		AddFinishData(vmOutput, []byte(finishString))
	}

	for i := recursiveCalls - 1; i >= 0; i-- {
		AddFinishData(vmOutput, []byte("succ"))
	}

	for i := 0; i <= recursiveCalls; i++ {
		key := fmt.Sprintf("Rkey%03d.........................", i)
		value := fmt.Sprintf("Rvalue%03d", i)
		SetStorageUpdateStrings(account, key, value)
	}

	SetStorageUpdate(account, recursiveIterationCounterKey, []byte{byte(recursiveCalls + 1)})
	SetStorageUpdate(account, recursiveIterationBigCounterKey, big.NewInt(int64(1)).Bytes())

	return vmOutput
}

func expectedVMOutputDestCtxRecursiveMutualMethods(_ []byte, recursiveCalls int) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	account := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	account.Balance = big.NewInt(1000)
	account.BalanceDelta = big.NewInt(0).Sub(big.NewInt(1), big.NewInt(1))
	account.GasUsed = 38083

	SetStorageUpdate(account, recursiveIterationCounterKey, []byte{byte(recursiveCalls + 1)})
	SetStorageUpdate(account, recursiveIterationBigCounterKey, big.NewInt(int64(1)).Bytes())

	AddFinishData(vmOutput, []byte("start recursive mutual calls"))

	for i := 0; i <= recursiveCalls; i++ {
		var finishData string
		var key string
		var value string
		iteration := recursiveCalls - i
		if i%2 == 0 {
			finishData = fmt.Sprintf("Afinish%03d", iteration)
			key = fmt.Sprintf("Akey%03d.........................", iteration)
			value = fmt.Sprintf("Avalue%03d", iteration)
		} else {
			finishData = fmt.Sprintf("Bfinish%03d", iteration)
			key = fmt.Sprintf("Bkey%03d.........................", iteration)
			value = fmt.Sprintf("Bvalue%03d", iteration)
		}
		SetStorageUpdateStrings(account, key, value)
		AddFinishData(vmOutput, []byte(finishData))
	}

	for i := recursiveCalls; i >= 0; i-- {
		AddFinishData(vmOutput, []byte("succ"))
	}

	AddFinishData(vmOutput, []byte("end recursive mutual calls"))

	return vmOutput
}

func expectedVMOutputDestCtxRecursiveMutualSCs(_ []byte, _ []byte, recursiveCalls int) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	parentIterations := (recursiveCalls / 2) + (recursiveCalls % 2)
	childIterations := recursiveCalls - parentIterations

	balanceDelta := int64(5*parentIterations - 3*childIterations)

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.BalanceDelta = big.NewInt(-balanceDelta)
	parentAccount.GasUsed = 7252

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		0,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)
	childAccount.BalanceDelta = big.NewInt(balanceDelta)
	childAccount.GasUsed = 5464

	for i := 0; i <= recursiveCalls; i++ {
		var finishData string
		var key string
		var value string
		iteration := recursiveCalls - i
		if i%2 == 0 {
			finishData = fmt.Sprintf("Pfinish%03d", iteration)
			key = fmt.Sprintf("Pkey%03d.........................", iteration)
			value = fmt.Sprintf("Pvalue%03d", iteration)
			SetStorageUpdateStrings(parentAccount, key, value)
		} else {
			finishData = fmt.Sprintf("Cfinish%03d", iteration)
			key = fmt.Sprintf("Ckey%03d.........................", iteration)
			value = fmt.Sprintf("Cvalue%03d", iteration)
			SetStorageUpdateStrings(childAccount, key, value)
		}
		AddFinishData(vmOutput, []byte(finishData))
	}

	for i := recursiveCalls - 1; i >= 0; i-- {
		AddFinishData(vmOutput, []byte("succ"))
	}

	counterValue := (recursiveCalls + recursiveCalls%2) / 2
	SetStorageUpdate(parentAccount, recursiveIterationCounterKey, []byte{byte(counterValue + 1)})
	SetStorageUpdate(childAccount, recursiveIterationCounterKey, []byte{byte(counterValue)})
	if recursiveCalls%2 == 0 {
		SetStorageUpdate(parentAccount, recursiveIterationBigCounterKey, big.NewInt(int64(1)).Bytes())
	} else {
		SetStorageUpdate(childAccount, recursiveIterationBigCounterKey, big.NewInt(int64(1)).Bytes())
	}

	return vmOutput
}

func expectedVMOutputDestCtxByCallerSimpleTransfer(value int64) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	parentAccount.Balance = nil
	parentAccount.GasUsed = 762

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		0,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)
	childAccount.BalanceDelta = big.NewInt(-value)
	childAccount.GasUsed = 667

	userAccount := AddNewOutputAccount(
		vmOutput,
		childAddress,
		userAddress,
		0,
		nil,
	)
	userAccount.BalanceDelta = big.NewInt(value)
	userAccount.OutputTransfers = append(userAccount.OutputTransfers, vmcommon.OutputTransfer{
		Value:         big.NewInt(value),
		GasLimit:      0,
		GasLocked:     0,
		Data:          []byte{},
		CallType:      vm.DirectCall,
		SenderAddress: childAddress,
	})

	AddFinishData(vmOutput, []byte("sent"))
	AddFinishData(vmOutput, []byte("child called"))
	return vmOutput
}

func expectedVMOutputAsyncCall(_ []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-10,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 104753
	SetStorageUpdate(parentAccount, parentKeyA, parentDataA)
	SetStorageUpdate(parentAccount, parentKeyB, parentDataB)
	AddFinishData(vmOutput, parentFinishA)
	AddFinishData(vmOutput, parentFinishB)

	thirdPartyAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		thirdPartyAddress,
		3,
		[]byte("hello"),
	)
	outTransfer := vmcommon.OutputTransfer{Data: []byte(" there"), Value: big.NewInt(3), SenderAddress: childAddress}
	thirdPartyAccount.OutputTransfers = append(thirdPartyAccount.OutputTransfers, outTransfer)
	thirdPartyAccount.BalanceDelta = big.NewInt(6)

	childAccount := AddNewOutputAccount(
		vmOutput,
		childAddress,
		childAddress,
		0,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)
	childAccount.GasUsed = 1296
	SetStorageUpdate(childAccount, childKey, childData)

	_ = AddNewOutputAccount(
		vmOutput,
		childAddress,
		vaultAddress,
		4,
		[]byte{},
	)

	AddFinishData(vmOutput, []byte{0})
	AddFinishData(vmOutput, []byte("thirdparty"))
	AddFinishData(vmOutput, []byte("vault"))
	AddFinishData(vmOutput, []byte{0})
	AddFinishData(vmOutput, []byte("succ"))

	return vmOutput
}

func expectedVMOutputAsyncCallChildFails(_ []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-7,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 3928
	SetStorageUpdate(parentAccount, parentKeyA, parentDataA)
	SetStorageUpdate(parentAccount, parentKeyB, parentDataB)
	AddFinishData(vmOutput, parentFinishA)
	AddFinishData(vmOutput, parentFinishB)

	_ = AddNewOutputAccount(
		vmOutput,
		parentAddress,
		thirdPartyAddress,
		3,
		[]byte("hello"),
	)

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		0,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)

	_ = AddNewOutputAccount(
		vmOutput,
		parentAddress,
		vaultAddress,
		4,
		[]byte{},
	)

	AddFinishData(vmOutput, []byte("succ"))

	return vmOutput
}

func expectedVMOutputAsyncCallCallBackFails(_ []byte, _ []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-10,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 197192
	SetStorageUpdate(parentAccount, parentKeyA, parentDataA)
	SetStorageUpdate(parentAccount, parentKeyB, parentDataB)
	AddFinishData(vmOutput, parentFinishA)
	AddFinishData(vmOutput, parentFinishB)

	_ = AddNewOutputAccount(
		vmOutput,
		parentAddress,
		thirdPartyAddress,
		3,
		[]byte("hello"),
	)
	outTransfer2 := vmcommon.OutputTransfer{Value: big.NewInt(3), Data: []byte(" there"), SenderAddress: childAddress}
	outAcc := vmOutput.OutputAccounts[string(thirdPartyAddress)]
	outAcc.OutputTransfers = append(outAcc.OutputTransfers, outTransfer2)
	outAcc.BalanceDelta = big.NewInt(6)

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		0,
		nil,
	)
	childAccount.Balance = big.NewInt(1000)
	childAccount.BalanceDelta = big.NewInt(0).Sub(big.NewInt(1), big.NewInt(1))
	childAccount.GasUsed = 1296
	SetStorageUpdate(childAccount, childKey, childData)

	_ = AddNewOutputAccount(
		vmOutput,
		childAddress,
		vaultAddress,
		4,
		[]byte{},
	)

	AddFinishData(vmOutput, []byte{3})
	AddFinishData(vmOutput, []byte("thirdparty"))
	AddFinishData(vmOutput, []byte("vault"))
	AddFinishData(vmOutput, []byte("user error"))
	AddFinishData(vmOutput, []byte("txhash"))

	vmOutput.ReturnMessage = "callBack error"

	return vmOutput
}

func expectedVMOutputCreateNewContractSuccess(_ []byte, childCode []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()
	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		-42,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.GasUsed = 885
	parentAccount.Nonce = 1
	SetStorageUpdate(parentAccount, []byte{'A'}, childCode)

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		[]byte("newAddress"),
		42,
		nil,
	)
	childAccount.Code = childCode
	childAccount.GasUsed = 472
	childAccount.CodeMetadata = []byte{1, 0}
	childAccount.CodeDeployerAddress = parentAddress

	l := len(childCode)
	AddFinishData(vmOutput, []byte{byte(l / 256), byte(l % 256)})
	AddFinishData(vmOutput, []byte("init successful"))
	AddFinishData(vmOutput, []byte("succ"))

	return vmOutput
}

func expectedVMOutputCreateNewContractFail(_ []byte, childCode []byte) *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()
	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	parentAccount.Nonce = 0
	parentAccount.GasUsed = 2885
	SetStorageUpdate(parentAccount, []byte{'A'}, childCode)

	l := len(childCode)
	AddFinishData(vmOutput, []byte{byte(l / 256), byte(l % 256)})
	AddFinishData(vmOutput, []byte("fail"))

	return vmOutput
}

func expectedVMOutputMockedWasmerInstances() *vmcommon.VMOutput {
	vmOutput := MakeVMOutput()

	parentAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		parentAddress,
		0,
		nil,
	)
	parentAccount.Balance = big.NewInt(1000)
	parentAccount.BalanceDelta = big.NewInt(-4)
	parentAccount.GasUsed = 547
	SetStorageUpdate(parentAccount, []byte("parent"), []byte("parent storage"))

	childAccount := AddNewOutputAccount(
		vmOutput,
		parentAddress,
		childAddress,
		0,
		nil,
	)
	childAccount.BalanceDelta = big.NewInt(4)
	childAccount.GasUsed = 146
	SetStorageUpdate(childAccount, []byte("child"), []byte("child storage"))

	AddFinishData(vmOutput, []byte("parent returns this"))
	AddFinishData(vmOutput, []byte("child returns this"))

	return vmOutput
}
