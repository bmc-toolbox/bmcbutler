package secrets

import (
	"fmt"
	"strings"
	"time"

	"github.com/bmc-toolbox/bmcbutler/pkg/config"
	vaultapi "github.com/hashicorp/vault/api"
)

type Store struct {
	data map[string]string
}

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

func (s *Store) Get(k string) (string, error) {
	value, exists := s.data[k]
	if !exists {
		return "", fmt.Errorf("Secret '%s' not found, has it been set in vault under vault.SecretsPath", k)
	}

	return value, nil
}

// UpdateConfigCredentials updates credentials that contain the lookup_secret keyword
func (s *Store) UpdateConfigCredentials(config []map[string]string) ([]map[string]string, error) {

	lookupPrefix := "lookup_secret::"
	// config is a []map[string]string
	for _, c := range config {
		for k, v := range c {
			if strings.HasPrefix(v, lookupPrefix) {

				lookup := strings.Replace(v, lookupPrefix, "", -1)
				if lookup == "" {
					return config, fmt.Errorf("config credentails key %s declares invalid lookup parameter", k)
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
