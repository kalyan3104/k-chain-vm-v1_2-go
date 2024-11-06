package mock

import (
	"math/big"

	"github.com/kalyan3104/k-chain-core-go/data/vm"
	vmcommon "github.com/kalyan3104/k-chain-vm-common-go"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/crypto"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/vmhost"
	"github.com/kalyan3104/k-chain-vm-v1_2-go/wasmer"
)

var _ vmhost.VMHost = (*VMHostStub)(nil)

// VMHostStub is used in tests to check the VMHost interface method calls
type VMHostStub struct {
	InitStateCalled       func()
	PushStateCalled       func()
	PopStateCalled        func()
	ClearStateStackCalled func()

	CryptoCalled                      func() crypto.VMCrypto
	BlockchainCalled                  func() vmhost.BlockchainContext
	RuntimeCalled                     func() vmhost.RuntimeContext
	BigIntCalled                      func() vmhost.BigIntContext
	OutputCalled                      func() vmhost.OutputContext
	MeteringCalled                    func() vmhost.MeteringContext
	StorageCalled                     func() vmhost.StorageContext
	RevertDCDTTransferCalled          func(input *vmcommon.ContractCallInput)
	ExecuteDCDTTransferCalled         func(destination []byte, sender []byte, tokenIdentifier []byte, nonce uint64, value *big.Int, callType vm.CallType, isRevert bool) (*vmcommon.VMOutput, uint64, error)
	CreateNewContractCalled           func(input *vmcommon.ContractCreateInput) ([]byte, error)
	ExecuteOnSameContextCalled        func(input *vmcommon.ContractCallInput) (*vmhost.AsyncContextInfo, error)
	ExecuteOnDestContextCalled        func(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, *vmhost.AsyncContextInfo, uint64, error)
	GetAPIMethodsCalled               func() *wasmer.Imports
	GetProtocolBuiltinFunctionsCalled func() vmcommon.FunctionNames
	IsBuiltinFunctionNameCalled       func(functionName string) bool
	AreInSameShardCalled              func(left []byte, right []byte) bool
}

// InitState mocked method
func (vhs *VMHostStub) InitState() {
	if vhs.InitStateCalled != nil {
		vhs.InitStateCalled()
	}
}

// PushState mocked method
func (vhs *VMHostStub) PushState() {
	if vhs.PushStateCalled != nil {
		vhs.PushStateCalled()
	}
}

// PopState mocked method
func (vhs *VMHostStub) PopState() {
	if vhs.PopStateCalled != nil {
		vhs.PopStateCalled()
	}
}

// ClearStateStack mocked method
func (vhs *VMHostStub) ClearStateStack() {
	if vhs.ClearStateStackCalled != nil {
		vhs.ClearStateStackCalled()
	}
}

// Crypto mocked method
func (vhs *VMHostStub) Crypto() crypto.VMCrypto {
	if vhs.CryptoCalled != nil {
		return vhs.CryptoCalled()
	}
	return nil
}

// Blockchain mocked method
func (vhs *VMHostStub) Blockchain() vmhost.BlockchainContext {
	if vhs.BlockchainCalled != nil {
		return vhs.BlockchainCalled()
	}
	return nil
}

// Runtime mocked method
func (vhs *VMHostStub) Runtime() vmhost.RuntimeContext {
	if vhs.RuntimeCalled != nil {
		return vhs.RuntimeCalled()
	}
	return nil
}

// BigInt mocked method
func (vhs *VMHostStub) BigInt() vmhost.BigIntContext {
	if vhs.BigIntCalled != nil {
		return vhs.BigIntCalled()
	}
	return nil
}

// IsVMV2Enabled mocked method
func (vhs *VMHostStub) IsVMV2Enabled() bool {
	return true
}

// IsVMV3Enabled mocked method
func (vhs *VMHostStub) IsVMV3Enabled() bool {
	return true
}

// IsAheadOfTimeCompileEnabled mocked method
func (vhs *VMHostStub) IsAheadOfTimeCompileEnabled() bool {
	return true
}

// IsDynamicGasLockingEnabled mocked method
func (vhs *VMHostStub) IsDynamicGasLockingEnabled() bool {
	return true
}

// IsDCDTFunctionsEnabled mocked method
func (vhs *VMHostStub) IsDCDTFunctionsEnabled() bool {
	return true
}

// Output mocked method
func (vhs *VMHostStub) Output() vmhost.OutputContext {
	if vhs.OutputCalled != nil {
		return vhs.OutputCalled()
	}
	return nil
}

// Metering mocked method
func (vhs *VMHostStub) Metering() vmhost.MeteringContext {
	if vhs.MeteringCalled != nil {
		return vhs.MeteringCalled()
	}
	return nil
}

// Storage mocked method
func (vhs *VMHostStub) Storage() vmhost.StorageContext {
	if vhs.StorageCalled != nil {
		return vhs.StorageCalled()
	}
	return nil
}

// RevertDCDTTransfer mocked method
func (vhs *VMHostStub) RevertDCDTTransfer(input *vmcommon.ContractCallInput) {
	if vhs.RevertDCDTTransferCalled != nil {
		vhs.RevertDCDTTransferCalled(input)
	}
}

// ExecuteDCDTTransfer mocked method
func (vhs *VMHostStub) ExecuteDCDTTransfer(destination []byte, sender []byte, tokenIdentifier []byte, nonce uint64, value *big.Int, callType vm.CallType, isRevert bool) (*vmcommon.VMOutput, uint64, error) {
	if vhs.ExecuteDCDTTransferCalled != nil {
		return vhs.ExecuteDCDTTransferCalled(destination, sender, tokenIdentifier, nonce, value, callType, isRevert)
	}
	return nil, 0, nil
}

// CreateNewContract mocked method
func (vhs *VMHostStub) CreateNewContract(input *vmcommon.ContractCreateInput) ([]byte, error) {
	if vhs.CreateNewContractCalled != nil {
		return vhs.CreateNewContractCalled(input)
	}
	return nil, nil
}

// ExecuteOnSameContext mocked method
func (vhs *VMHostStub) ExecuteOnSameContext(input *vmcommon.ContractCallInput) (*vmhost.AsyncContextInfo, error) {
	if vhs.ExecuteOnSameContextCalled != nil {
		return vhs.ExecuteOnSameContextCalled(input)
	}
	return nil, nil
}

// ExecuteOnDestContext mocked method
func (vhs *VMHostStub) ExecuteOnDestContext(input *vmcommon.ContractCallInput) (*vmcommon.VMOutput, *vmhost.AsyncContextInfo, uint64, error) {
	if vhs.ExecuteOnDestContextCalled != nil {
		return vhs.ExecuteOnDestContextCalled(input)
	}
	return nil, nil, 0, nil
}

// AreInSameShard mocked method
func (vhs *VMHostStub) AreInSameShard(left []byte, right []byte) bool {
	if vhs.AreInSameShardCalled != nil {
		return vhs.AreInSameShardCalled(left, right)
	}
	return true
}

// GetAPIMethods mocked method
func (vhs *VMHostStub) GetAPIMethods() *wasmer.Imports {
	if vhs.GetAPIMethodsCalled != nil {
		return vhs.GetAPIMethodsCalled()
	}
	return nil
}

// GetProtocolBuiltinFunctions mocked method
func (vhs *VMHostStub) GetProtocolBuiltinFunctions() vmcommon.FunctionNames {
	if vhs.GetProtocolBuiltinFunctionsCalled != nil {
		return vhs.GetProtocolBuiltinFunctionsCalled()
	}
	return make(vmcommon.FunctionNames)
}

// IsBuiltinFunctionName mocked method
func (vhs *VMHostStub) IsBuiltinFunctionName(functionName string) bool {
	if vhs.IsBuiltinFunctionNameCalled != nil {
		return vhs.IsBuiltinFunctionNameCalled(functionName)
	}
	return false
}
