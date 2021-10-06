package config

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Load sets up bmcbutler configuration.
// nolint: gocyclo
func (p *Params) Load(cfgFile string) {
	_, err := os.Stat(cfgFile)
	if err != nil {
		log.Fatalf("[Error] unable to read bmcbutler config file: %s", err)
	}

	err = p.unmarshalConfig(cfgFile)
	if err != nil {
		log.Fatalf("failed to unmarshal config: %s", err.Error())
	}

	validators := []func() error{
		p.validateVaultCfg,
		p.validateMetricsCfg,
		p.validateInventoryCfg,
		p.defaults,
		p.validateCertSignerCfg,
	}

	for _, v := range validators {
		err := v()
		if err != nil {
			log.Fatalf("[Error] %s", err.Error())
		}
	}
}

// Read the config file using Viper.
func (p *Params) unmarshalConfig(cfgFile string) error {
	if cfgFile != "" {
		// Use config file from the user-provided flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("bmcbutler")
		viper.AddConfigPath("/etc/bmcbutler")
	}

	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	err = viper.Unmarshal(&p)
	if err != nil {
		return err
	}

	return nil
}

func (p *Params) defaults() error {
	// Minimum butlers to spawn.
	if p.ButlersToSpawn == 0 {
		p.ButlersToSpawn = 5
	}

	if p.Credentials == nil {
		log.Println("[Error] Expected BMC credentials to be declared in configuration")
		os.Exit(1)
	}

	return nil
}

func (p *Params) validateCertSignerCfg() error {
	if p.CertSigner != nil {
		if p.CertSigner.FakeSigner != nil {
			p.CertSigner.Client = "fakeSigner"
		} else if p.CertSigner.LemurSigner != nil {
			p.CertSigner.Client = "lemurSigner"
		} else {
			log.Println("[WARN] Invalid cert_signer declared in config.")
		}
	}

	return nil
}

func (p *Params) validateInventoryCfg() error {
	if p.Inventory != nil {
		if p.Inventory.Enc != nil {
			p.Inventory.Source = "enc"
		} else if p.Inventory.Dora != nil {
			p.Inventory.Source = "dora"
		} else if p.Inventory.Csv != nil {
			p.Inventory.Source = "csv"
		} else {
			log.Println("[WARN] Invalid inventory source declared in configuration.")
		}
	}

	return nil
}

// metrics config
func (p *Params) validateMetricsCfg() error {
	if p.Metrics != nil {
		if p.Metrics.Graphite != nil {
			p.Metrics.Client = "graphite"
		} else {
			log.Println("[WARN] Invalid metrics client declared in config.")
		}
	}

	return nil
}

// vault config
func (p *Params) validateVaultCfg() error {
	if !p.SecretsFromVault {
		return nil
	}

	if p.Vault == nil {
		return fmt.Errorf("secretsFromVault declared, expected vault configuration section missing")
	}

	if p.Vault.HostAddress == "" {
		return fmt.Errorf("bmcbutler vault configuration expects a valid hostAddress")
	}

	if p.Vault.SecretsPath == "" {
		return fmt.Errorf("bmcbutler vault configuration expects the vault path for secrets")
	}

	err := p.loadVaultToken()
	if err != nil {
		return err
	}

	return nil
}

func (p *Params) loadVaultToken() error {
	// token declared in config file
	if p.Vault.Token != "" {
		return nil
	}

	// token declared in the env variable
	if p.Vault.TokenFromEnv {
		token := os.Getenv("VAULT_TOKEN")
		if len(token) < 5 {
			return fmt.Errorf("Vault token load from env var VAULT_TOKEN failed - empty/token invalid")
		}

		p.Vault.Token = token
		return nil
	}

	// token declared in a file
	if p.Vault.TokenFromFile != "" {
		b, err := ioutil.ReadFile(p.Vault.TokenFromFile)
		if err != nil {
			return fmt.Errorf("Vault token load from file %s failed: %s",
				p.Vault.TokenFromFile,
				err.Error())
		}

		if len(b) < 5 {
			return fmt.Errorf("Vault token load from file %s: file empty/token invalid", p.Vault.TokenFromFile)
		}

		p.Vault.Token = strings.Replace(string(b), "\n", "", -1)
		return nil
	}

	return fmt.Errorf("expected vault.tokenFromFile OR vault.token OR vault.tokenFromEnv to be declared")
}
