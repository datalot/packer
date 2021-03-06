package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/packer/packer"
)

func TestDecodeConfig(t *testing.T) {

	packerConfig := `
	{
		"PluginMinPort": 10,
		"PluginMaxPort": 25,
		"disable_checkpoint": true,
		"disable_checkpoint_signature": true,
		"provisioners": {
		    "super-shell": "packer-provisioner-super-shell"
		}
	}`

	var cfg config
	err := decodeConfig(strings.NewReader(packerConfig), &cfg)
	if err != nil {
		t.Fatalf("error encountered decoding configuration: %v", err)
	}

	var expectedCfg config
	json.NewDecoder(strings.NewReader(packerConfig)).Decode(&expectedCfg)
	if !reflect.DeepEqual(cfg, expectedCfg) {
		t.Errorf("failed to load custom configuration data; expected %v got %v", expectedCfg, cfg)
	}

}

func TestLoadExternalComponentsFromConfig(t *testing.T) {
	packerConfigData, cleanUpFunc, err := generateFakePackerConfigData()
	if err != nil {
		t.Fatalf("error encountered while creating fake Packer configuration data %v", err)
	}
	defer cleanUpFunc()

	var cfg config
	cfg.Builders = packer.MapOfBuilder{}
	cfg.PostProcessors = packer.MapOfPostProcessor{}
	cfg.Provisioners = packer.MapOfProvisioner{}

	if err := decodeConfig(strings.NewReader(packerConfigData), &cfg); err != nil {
		t.Fatalf("error encountered decoding configuration: %v", err)
	}

	cfg.LoadExternalComponentsFromConfig()

	if len(cfg.Builders) != 1 || !cfg.Builders.Has("cloud-xyz") {
		t.Errorf("failed to load external builders; got %v as the resulting config", cfg.Builders)
	}

	if len(cfg.Provisioners) != 1 || !cfg.Provisioners.Has("super-shell") {
		t.Errorf("failed to load external provisioners; got %v as the resulting config", cfg.Provisioners)
	}

	if len(cfg.PostProcessors) != 1 || !cfg.PostProcessors.Has("noop") {
		t.Errorf("failed to load external post-processors; got %v as the resulting config", cfg.PostProcessors)
	}
}

func TestLoadExternalComponentsFromConfig_onlyProvisioner(t *testing.T) {
	packerConfigData, cleanUpFunc, err := generateFakePackerConfigData()
	if err != nil {
		t.Fatalf("error encountered while creating fake Packer configuration data %v", err)
	}
	defer cleanUpFunc()

	var cfg config
	cfg.Provisioners = packer.MapOfProvisioner{}

	if err := decodeConfig(strings.NewReader(packerConfigData), &cfg); err != nil {
		t.Fatalf("error encountered decoding configuration: %v", err)
	}

	/* Let's clear out any custom Builders or PostProcessors that were part of the config.
	This step does not remove them from disk, it just removes them from of plugins Packer knows about.
	*/
	cfg.RawBuilders = nil
	cfg.RawPostProcessors = nil

	cfg.LoadExternalComponentsFromConfig()

	if len(cfg.Builders) != 0 || cfg.Builders.Has("cloud-xyz") {
		t.Errorf("loaded external builders when it wasn't supposed to; got %v as the resulting config", cfg.Builders)
	}

	if len(cfg.Provisioners) != 1 || !cfg.Provisioners.Has("super-shell") {
		t.Errorf("failed to load external provisioners; got %v as the resulting config", cfg.Provisioners)
	}

	if len(cfg.PostProcessors) != 0 || cfg.PostProcessors.Has("noop") {
		t.Errorf("loaded external post-processors when it wasn't supposed to; got %v as the resulting config", cfg.PostProcessors)
	}
}

/* generateFakePackerConfigData creates a collection of mock plugins along with a basic packerconfig.
The return packerConfigData is a valid packerconfig file that can be used for configuring external plugins, cleanUpFunc is a function that should be called for cleaning up any generated mock data.
This function will only clean up if there is an error, on successful runs the caller
is responsible for cleaning up the data via cleanUpFunc().
*/
func generateFakePackerConfigData() (packerConfigData string, cleanUpFunc func(), err error) {
	dir, err := ioutil.TempDir("", "random-testdata")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary test directory: %v", err)
	}

	cleanUpFunc = func() {
		os.RemoveAll(dir)
	}

	var suffix string
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}

	plugins := [...]string{
		filepath.Join(dir, "packer-builder-cloud-xyz"+suffix),
		filepath.Join(dir, "packer-provisioner-super-shell"+suffix),
		filepath.Join(dir, "packer-post-processor-noop"+suffix),
	}
	for _, plugin := range plugins {
		_, err := os.Create(plugin)
		if err != nil {
			cleanUpFunc()
			return "", nil, fmt.Errorf("failed to create temporary plugin file (%s): %v", plugin, err)
		}
	}

	packerConfigData = fmt.Sprintf(`
	{
		"PluginMinPort": 10,
		"PluginMaxPort": 25,
		"disable_checkpoint": true,
		"disable_checkpoint_signature": true,
		"builders": {
			"cloud-xyz": %q
		},
		"provisioners": {
			"super-shell": %q
		},
		"post-processors": {
			"noop": %q
		}
	}`, plugins[0], plugins[1], plugins[2])

	return
}
