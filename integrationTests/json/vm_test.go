package vmjsonintegrationtest

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	logger "github.com/kalyan3104/k-chain-logger-go"
	am "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarioexec"
	mc "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/controller"
	"github.com/stretchr/testify/require"
)

func init() {
	_ = logger.SetLogLevel("*:DEBUG")
}

func getTestRoot() string {
	exePath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	vmTestRoot := filepath.Join(exePath, "../../test")
	return vmTestRoot
}

// Tests Scenarios consistency, no smart contracts.
func TestScenariosSelfTest(t *testing.T) {
	runTestsInFolder(t, "scenarios-self-test", []string{
		"scenarios-self-test/builtin-func-dcdt-transfer.scen.json",
	})
}

func TestRustErc20(t *testing.T) {
	runAllTestsInFolder(t, "erc20-rust/scenarios")
}

func TestCErc20(t *testing.T) {
	runAllTestsInFolder(t, "erc20-c")
}

func TestRustAdder(t *testing.T) {
	runAllTestsInFolder(t, "adder/scenarios")
}

func TestMultisig(t *testing.T) {
	runAllTestsInFolder(t, "multisig/scenarios")
}

func TestRustBasicFeaturesLatest(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	runAllTestsInFolder(t, "features/basic-features/scenarios")
}

func TestRustBasicFeaturesNoSmallIntApi(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	runAllTestsInFolder(t, "features/basic-features-no-small-int-api/scenarios")
}

// Backwards compatibility.
func TestRustBasicFeaturesLegacy(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	runAllTestsInFolder(t, "features/basic-features-legacy/scenarios")
}

// func TestRustPayableFeaturesLatest(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("not a short test")
// 	}

// 	runAllTestsInFolder(t, "features/payable-features/scenarios")
// }

// func TestRustAsyncCalls(t *testing.T) {
// 	runTestsInFolder(t, "features/async/scenarios", []string{
// 		"features/async/scenarios/forwarder_sync_accept_dcdt.scen.json",
// 		"features/async/scenarios/forwarder_send_twice_dcdt.scen.json",
// 		"features/async/scenarios/recursive_caller_dcdt_1.scen.json",
// 		"features/async/scenarios/recursive_caller_dcdt_2.scen.json",
// 		"features/async/scenarios/recursive_caller_dcdt_x.scen.json",
// 	})
// }

func TestDelegation_v0_2(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	runAllTestsInFolder(t, "delegation/v0_2")
}

// func TestDelegation_v0_3(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("not a short test")
// 	}

// 	runTestsInFolder(t, "delegation/v0_3", []string{
// 		"delegation/v0_3/test/integration/genesis/genesis.scen.json",
// 	})
// }

func TestDelegation_v0_4_genesis(t *testing.T) {
	if testing.Short() {
		t.Skip("not a short test")
	}

	runAllTestsInFolder(t, "delegation/v0_4_genesis")
}

// func TestDelegation_v0_5_latest_full(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("not a short test")
// 	}

// 	runAllTestsInFolder(t, "delegation/v0_5_latest_full")
// }

// func TestDelegation_v0_5_latest_update(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("not a short test")
// 	}

// 	runAllTestsInFolder(t, "delegation/v0_5_latest_update")
// }

// func TestDnsContract(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("not a short test")
// 	}

// 	runAllTestsInFolder(t, "dns")
// }

func TestTimelocks(t *testing.T) {
	runAllTestsInFolder(t, "timelocks")
}

// func TestPromises(t *testing.T) {
// 	executor, err := am.NewVMTestExecutor()
// 	require.Nil(t, err)
// 	runner := mc.NewScenarioRunner(
// 		executor,
// 		mc.NewDefaultFileResolver(),
// 	)
// 	err = runner.RunAllJSONScenariosInDirectory(
// 		getTestRoot(),
// 		"promises",
// 		".scen.json",
// 		[]string{})

// 	if err != nil {
// 		t.Error(err)
// 	}
// }

// func TestCrowdfundingDcdt(t *testing.T) {
// 	runAllTestsInFolder(t, "crowdfunding-dcdt")
// }

// func TestRewaDcdtSwap(t *testing.T) {
// 	runAllTestsInFolder(t, "rewa-dcdt-swap")
// }

// func TestPingPongRewa(t *testing.T) {
// 	runAllTestsInFolder(t, "ping-pong-rewa")
// }

func runAllTestsInFolder(t *testing.T, folder string) {
	runTestsInFolder(t, folder, []string{})
}

func runTestsInFolder(t *testing.T, folder string, exclusions []string) {
	executor, err := am.NewVMTestExecutor("../../scenarioexec")
	require.Nil(t, err)
	runner := mc.NewScenarioRunner(
		executor,
		mc.NewDefaultFileResolver(),
	)

	err = runner.RunAllJSONScenariosInDirectory(
		getTestRoot(),
		folder,
		".scen.json",
		exclusions)

	if err != nil {
		t.Error(err)
	}
}

func runSingleTest(t *testing.T, folder string, filename string) {
	executor, err := am.NewVMTestExecutor("../../scenarioexec")
	require.Nil(t, err)
	runner := mc.NewScenarioRunner(
		executor,
		mc.NewDefaultFileResolver(),
	)

	fullPath := path.Join(getTestRoot(), folder)
	fullPath = path.Join(fullPath, filename)

	err = runner.RunSingleJSONScenario(fullPath)
	if err != nil {
		t.Error(err)
	}
}
