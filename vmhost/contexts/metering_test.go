package contexts

import (
	"math/big"
	"testing"

	"github.com/kalyan3104/k-chain-core-go/data/vm"
	vmcommon "github.com/kalyan3104/k-chain-vm-common-go"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/config"
	contextmock "github.com/kalyan3104/k-chain-vm-v1_2-go/mock/context"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/vmhost"
	"github.com/stretchr/testify/require"
)

func TestNewMeteringContext(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{}

	meteringContext, err := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))
	require.Nil(t, err)
	require.NotNil(t, meteringContext)
}

func TestNewMeteringContext_NilGasSchedule(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostMock{}

	meteringContext, err := NewMeteringContext(host, nil, uint64(15000))
	require.NotNil(t, err)
	require.Nil(t, meteringContext)
}

func TestMeteringContext_GasSchedule(t *testing.T) {
	t.Parallel()

	host := &contextmock.VMHostStub{}
	meteringContext, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	schedule := meteringContext.GasSchedule()
	require.NotNil(t, schedule)
}

func TestMeteringContext_UseGas(t *testing.T) {
	t.Parallel()

	mockRuntime := &contextmock.RuntimeContextMock{}
	host := &contextmock.VMHostMock{
		RuntimeContext: mockRuntime,
	}
	meteringContext, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	gasProvided := uint64(1001)
	meteringContext.gasForExecution = gasProvided
	gas := uint64(1000)
	meteringContext.UseGas(gas)
	require.Equal(t, mockRuntime.GetPointsUsed(), gas)
	require.Equal(t, uint64(1), meteringContext.GasLeft())

	gasProvided = uint64(10000)
	mockRuntime.SetPointsUsed(0)
	meteringContext, _ = NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))
	meteringContext.gasForExecution = gasProvided

	require.Equal(t, gasProvided, meteringContext.GasLeft())
	meteringContext.UseGas(gas)
	require.Equal(t, gasProvided-gas, meteringContext.GasLeft())
}

func TestMeteringContext_FreeGas(t *testing.T) {
	t.Parallel()

	mockOutput := &contextmock.OutputContextMock{}
	host := &contextmock.VMHostMock{
		OutputContext: mockOutput,
	}

	meteringContext, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	mockOutput.GasRefund = big.NewInt(0)
	meteringContext.FreeGas(1000)
	gas := mockOutput.GetRefund()
	require.Equal(t, uint64(1000), gas)

	meteringContext.FreeGas(100)
	gas = mockOutput.GetRefund()
	require.Equal(t, uint64(1100), gas)
}

func TestMeteringContext_BoundGasLimit(t *testing.T) {
	t.Parallel()

	mockRuntime := &contextmock.RuntimeContextMock{}
	host := &contextmock.VMHostMock{
		RuntimeContext: mockRuntime,
	}
	blockGasLimit := uint64(15000)
	meteringContext, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	gasProvided := uint64(10000)
	meteringContext.gasForExecution = gasProvided
	mockRuntime.SetPointsUsed(0)

	gasLimit := 5000
	limit := meteringContext.BoundGasLimit(int64(gasLimit))
	require.Equal(t, uint64(gasLimit), limit)

	gasLimit = 25000
	limit = meteringContext.BoundGasLimit(int64(gasLimit))
	require.Equal(t, meteringContext.GasLeft(), limit)

	blockLimit := meteringContext.BlockGasLimit()
	require.Equal(t, blockGasLimit, blockLimit)
}

func TestMeteringContext_DeductInitialGasForExecution(t *testing.T) {
	t.Parallel()

	mockRuntime := &contextmock.RuntimeContextMock{}
	gasProvided := uint64(10000)
	vmInput := &vmcommon.VMInput{
		GasProvided: gasProvided,
	}

	mockRuntime.SetVMInput(vmInput)

	host := &contextmock.VMHostMock{
		RuntimeContext: mockRuntime,
	}

	meteringContext, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	contract := []byte("contract")
	err := meteringContext.DeductInitialGasForExecution(contract)
	require.Nil(t, err)

	vmInput.GasProvided = 1
	err = meteringContext.DeductInitialGasForExecution(contract)
	require.Equal(t, vmhost.ErrNotEnoughGas, err)
}

func TestDeductInitialGasForDirectDeployment(t *testing.T) {
	t.Parallel()
	mockRuntime := &contextmock.RuntimeContextMock{}
	gasProvided := uint64(10000)
	contractCode := []byte("contractCode")
	input := &vmcommon.ContractCreateInput{
		VMInput: vmcommon.VMInput{
			GasProvided: gasProvided,
		},
		ContractCode: contractCode,
	}

	mockRuntime.SetVMInput(&input.VMInput)

	host := &contextmock.VMHostMock{
		RuntimeContext: mockRuntime,
	}

	meteringContext, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	mockRuntime.SetPointsUsed(0)
	err := meteringContext.DeductInitialGasForDirectDeployment(vmhost.CodeDeployInput{ContractCode: contractCode})
	require.Nil(t, err)
	remainingGas := meteringContext.GasLeft()
	require.Equal(t, gasProvided-uint64(len(contractCode))-1, remainingGas)

	input.GasProvided = 2
	mockRuntime.SetPointsUsed(0)
	err = meteringContext.DeductInitialGasForDirectDeployment(vmhost.CodeDeployInput{ContractCode: contractCode})
	require.Equal(t, vmhost.ErrNotEnoughGas, err)
}

func TestDeductInitialGasForIndirectDeployment(t *testing.T) {
	t.Parallel()

	mockRuntime := &contextmock.RuntimeContextMock{}
	gasProvided := uint64(10000)
	contractCode := []byte("contractCode")
	input := &vmcommon.ContractCreateInput{
		VMInput: vmcommon.VMInput{
			GasProvided: gasProvided,
		},
		ContractCode: contractCode,
	}

	mockRuntime.SetVMInput(&input.VMInput)

	host := &contextmock.VMHostMock{
		RuntimeContext: mockRuntime,
	}

	meteringContext, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	mockRuntime.SetPointsUsed(0)
	err := meteringContext.DeductInitialGasForIndirectDeployment(vmhost.CodeDeployInput{ContractCode: contractCode})
	require.Nil(t, err)
	remainingGas := meteringContext.GasLeft()
	require.Equal(t, gasProvided-uint64(len(contractCode)), remainingGas)

	input.GasProvided = 2
	mockRuntime.SetPointsUsed(0)
	err = meteringContext.DeductInitialGasForDirectDeployment(vmhost.CodeDeployInput{ContractCode: contractCode})
	require.Equal(t, vmhost.ErrNotEnoughGas, err)
}

func TestMeteringContext_AsyncCallGasLocking(t *testing.T) {
	t.Parallel()

	mockRuntime := &contextmock.RuntimeContextMock{}
	contractSize := uint64(1000)
	input := &vmcommon.ContractCallInput{
		VMInput: vmcommon.VMInput{
			CallType: vm.AsynchronousCall,
		},
	}

	mockRuntime.SCCodeSize = contractSize
	mockRuntime.SetVMInput(&input.VMInput)
	mockRuntime.SetPointsUsed(0)

	host := &contextmock.VMHostMock{
		RuntimeContext: mockRuntime,
	}

	meteringContext, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	input.GasProvided = 1
	err := meteringContext.UseGasForAsyncStep()
	require.Equal(t, vmhost.ErrNotEnoughGas, err)

	mockRuntime.SetPointsUsed(0)
	gasProvided := uint64(1_000_000)
	input.GasProvided = gasProvided
	meteringContext.gasForExecution = gasProvided
	gasToLock := meteringContext.ComputeGasLockedForAsync()
	err = meteringContext.UseGasBounded(gasToLock)
	require.Nil(t, err)
	expectedGasLeft := gasProvided - gasToLock
	require.Equal(t, expectedGasLeft, meteringContext.GasLeft())

	mockRuntime.VMInput.CallType = vm.AsynchronousCallBack
	mockRuntime.VMInput.GasLocked = gasToLock
	meteringContext.unlockGasIfAsyncCallback(&input.VMInput)
	err = meteringContext.UseGasForAsyncStep()
	require.Nil(t, err)
	require.Equal(t, gasProvided-1, meteringContext.GasLeft())
}

func TestMeteringContext_GasUsed_NoStacking(t *testing.T) {
	t.Parallel()

	mockRuntime := &contextmock.RuntimeContextMock{}
	host := &contextmock.VMHostMock{
		RuntimeContext: mockRuntime,
	}

	contractSize := uint64(1000)
	contract := make([]byte, contractSize)
	input := &vmcommon.ContractCallInput{VMInput: vmcommon.VMInput{}}

	mockRuntime.SCCodeSize = contractSize
	mockRuntime.SetVMInput(&input.VMInput)
	mockRuntime.SetPointsUsed(0)

	metering, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	input.GasProvided = 2000
	metering.InitStateFromContractCallInput(&input.VMInput)
	require.Equal(t, uint64(2000), metering.initialGasProvided)

	_ = metering.DeductInitialGasForExecution(contract)
	require.Equal(t, uint64(999), metering.GasLeft())

	metering.UseGas(400)
	require.Equal(t, uint64(599), metering.GasLeft())

	gasUsedByContract, _ := metering.GasUsedByContract()
	require.Equal(t, uint64(1401), gasUsedByContract)
}

func TestMeteringContext_GasUsed_StackOneLevel(t *testing.T) {
	t.Parallel()

	mockRuntime := &contextmock.RuntimeContextMock{}
	host := &contextmock.VMHostMock{
		RuntimeContext: mockRuntime,
	}

	contractSize := uint64(1000)
	contract := make([]byte, contractSize)
	mockRuntime.SCCodeSize = contractSize
	mockRuntime.SCAddress = []byte("parent")

	mockRuntime.SetPointsUsed(0)
	parentInput := &vmcommon.ContractCallInput{VMInput: vmcommon.VMInput{}}
	mockRuntime.SetVMInput(&parentInput.VMInput)

	metering, _ := NewMeteringContext(host, config.MakeGasMapForTests(), uint64(15000))

	parentInput.GasProvided = 4000
	metering.InitStateFromContractCallInput(&parentInput.VMInput)
	require.Equal(t, uint64(4000), metering.initialGasProvided)

	_ = metering.DeductInitialGasForExecution(contract)
	require.Equal(t, uint64(2999), metering.GasLeft())

	metering.UseGas(400)
	require.Equal(t, uint64(2599), metering.GasLeft())

	gasUsedByContract, _ := metering.GasUsedByContract()
	require.Equal(t, uint64(1401), gasUsedByContract)

	// simulate executing another contract on top of the parent
	childInput := &vmcommon.ContractCallInput{VMInput: vmcommon.VMInput{}}
	childInput.GasProvided = 500

	metering.UseGas(childInput.GasProvided)
	parentPointsBeforeStacking := mockRuntime.GetPointsUsed()

	// child execution begins
	mockRuntime.SCAddress = []byte("child")
	mockRuntime.SetPointsUsed(0)
	mockRuntime.SetVMInput(&childInput.VMInput)
	metering.PushState()
	metering.InitStateFromContractCallInput(&childInput.VMInput)
	require.Equal(t, uint64(500), metering.initialGasProvided)

	_ = metering.DeductInitialGasForExecution(make([]byte, 100))
	require.Equal(t, uint64(399), metering.GasLeft())

	metering.UseGas(50)
	gasRemaining := metering.GasLeft()
	require.Equal(t, uint64(349), gasRemaining)

	gasUsedByContract, _ = metering.GasUsedByContract()
	require.Equal(t, uint64(151), gasUsedByContract)

	// return to the parent
	mockRuntime.SCAddress = []byte("parent")
	metering.PopSetActiveState()
	mockRuntime.SetPointsUsed(parentPointsBeforeStacking)
	mockRuntime.SetVMInput(&parentInput.VMInput)

	metering.RestoreGas(gasRemaining)
	mockRuntime.IsContractOnStack = false
	metering.ForwardGas([]byte("parent"), []byte("child"), gasUsedByContract)
	require.Equal(t, uint64(2448), metering.GasLeft())

	gasUsedByContract, _ = metering.GasUsedByContract()
	require.Equal(t, uint64(1401), gasUsedByContract)

	metering.UseGas(50)
	require.Equal(t, uint64(2398), metering.GasLeft())

	gasUsedByContract, _ = metering.GasUsedByContract()
	require.Equal(t, uint64(1451), gasUsedByContract)
}
