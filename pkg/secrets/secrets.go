package secrets

import (
	"fmt"
	"strings"
	"time"

	"github.com/bmc-toolbox/bmcbutler/pkg/config"
	vaultapi "github.com/hashicorp/vault/api"
)

// Store holds a copy of secrets from vault
type Store struct {
	data map[string]string
}

// Load connects to Vault and returns a secret Store populated with secrets
func Load(c config.Vault) (*Store, error) {
	s := &Store{data: make(map[string]string)}
	v, err := vaultapi.NewClient(
		&vaultapi.Config{
			Address:    c.HostAddress,
			Timeout:    20 * time.Second,
			MaxRetries: 5,
		},
	)
	if err != nil {
		return s, err
	}

	v.SetToken(c.Token)
	secrets, err := v.Logical().Read(c.SecretsPath)
	if err != nil {
		return s, err
	}

	if secrets == nil {
		return s, fmt.Errorf("read on vault secrets path %s returned nil", c.SecretsPath)
	}

	for k, v := range secrets.Data {
		s.data[k] = v.(string)
	}

	return s, nil
}

// Get retrieves a secret based on the given key
func (s *Store) Get(k string) (string, error) {
	value, exists := s.data[k]
	if !exists {
		return "", fmt.Errorf("Secret '%s' not found, has it been set in vault under vault.SecretsPath", k)
	}

	return value, nil
}

// GetSignerToken is a helper method to retrieve and return the signer token key
func (s *Store) GetSignerToken(v string) (string, error) {
	prefix := "lookup_secret::"

	if !strings.HasPrefix(v, prefix) {
		return "", fmt.Errorf("expected prefix lookup_secret:: for signer token, got: %s", v)
	}

	lookup := strings.TrimPrefix(v, prefix)
	if lookup == "" {
		return "", fmt.Errorf("signer token value %s declares invalid lookup parameter", v)
	}

	secret, err := s.Get(lookup)
	if err != nil {
		return secret, err
	}

	return secret, nil
}

// SetCredentials updates credentials that contain the lookup_secret keyword
func (s *Store) SetCredentials(config []map[string]string) ([]map[string]string, error) {
	lookupPrefix := "lookup_secret::"
	// config is a []map[string]string
	for _, c := range config {
		for k, v := range c {
			if strings.HasPrefix(v, lookupPrefix) {

				lookup := strings.Replace(v, lookupPrefix, "", -1)
				if lookup == "" {
					return config, fmt.Errorf("config credentials key %s declares invalid lookup parameter", k)
				}

				secret, err := s.Get(lookup)
				if err != nil {
					return config, err
				}

				c[k] = secret
			}
		}
	}

	return config, nil
}
