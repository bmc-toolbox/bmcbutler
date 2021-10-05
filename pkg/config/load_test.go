package config

import (
	"fmt"
	"testing"
)

func initTestConfig() (*Params, error) {
	// source config file
	config := "../../samples/bmcbutler.yml"
	params := &Params{}
	err := params.unmarshalConfig(config)
	if err != nil {
		return params, err
	}

	return params, nil
}

func TestLoadVaultToken(t *testing.T) {
	cfg, err := initTestConfig()
	if err != nil {
		t.Errorf("Error loading test config: %s", err.Error())
	}

	if err := cfg.loadVaultToken(); err != nil {
		t.Errorf("Expected to load a vault token: %s", err.Error())
	}

	if cfg.Vault.Token == "" {
		t.Error("Expected to load a valid vault token, got empty value")
	}

	fmt.Println(cfg.Vault.Token)
}

func TestLoad(t *testing.T) {
	cfg, err := initTestConfig()
	if err != nil {
		t.Errorf("Error loading test config: %s", err.Error())
	}

	config := "../../samples/bmcbutler.yml"
	cfg.Load(config)

	if cfg.ButlersToSpawn != 1 {
		t.Errorf("Expected ButlersToSpawn: 1, got %d", cfg.ButlersToSpawn)
	}
}
