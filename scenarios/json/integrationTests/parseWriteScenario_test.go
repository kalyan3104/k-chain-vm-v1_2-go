package scenjsontest

import (
	"io/ioutil"
	"testing"

	fr "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/fileresolver"
	mjparse "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/json/parse"
	mjwrite "github.com/kalyan3104/k-chain-vm-v1_2-go/scenarios/json/write"
	"github.com/stretchr/testify/require"
)

func TestWriteScenario(t *testing.T) {
	contents, err := loadExampleFile("example.scen.json")
	require.Nil(t, err)

	p := mjparse.NewParser(
		fr.NewDefaultFileResolver().ReplacePath(
			"smart-contract.wasm",
			"exampleFile.txt"))

	scenario, parseErr := p.ParseScenarioFile(contents)
	require.Nil(t, parseErr)

	serialized := mjwrite.ScenarioToJSONString(scenario)

	// good for debugging:
	_ = ioutil.WriteFile("serialized.scen.json", []byte(serialized), 0644)

	require.Equal(t, contents, []byte(serialized))
}