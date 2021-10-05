// Copyright © 2018 Joel Rebello <joel.rebello@booking.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"time"
)

// Params struct holds all bmcbutler configuration parameters
type Params struct {
	ButlersToSpawn   int                 `mapstructure:"butlersToSpawn"`
	Credentials      []map[string]string `mapstructure:"credentials"`
	CertSigner       *CertSigner         `mapstructure:"cert_signer"`
	Inventory        *Inventory          `mapstructure:"inventory"`
	Locations        []string            `mapstructure:"locations"`
	Metrics          *Metrics            `mapstructure:"metrics"`
	FilterParams     *FilterParams
	CfgFile          string
	Configure        bool // The user invoked the configure action?
	DryRun           bool // If true, don't carry out any actions. Just log.
	Execute          bool // The user invoked the execute action?
	IgnoreLocation   bool
	Resources        []string
	Version          string
	Debug            bool
	Trace            bool
	SecretsFromVault bool   `mapstructure:"secretsFromVault"`
	Vault            *Vault `mapstructure:"vault"`
}

// Inventory struct holds inventory configuration parameters.
type Inventory struct {
	Source string // dora, csv, enc
	Enc    *Enc   `mapstructure:"enc"`
	Dora   *Dora  `mapstructure:"dora"`
	Csv    *Csv   `mapstrucure:"csv"`
}

// Enc declares config for a ENC as an inventory source
type Enc struct {
	Bin          string   `mapstructure:"bin"`
	BMCNicPrefix []string `mapstructure:"bmcNicPrefix"`
}

// Csv declares config for a CSV file as an inventory source
type Csv struct {
	File string `mapstructure:"file"`
}

// Dora declares config for Dora as a inventory source.
type Dora struct {
	URL string `mapstructure:"url"`
}

// Metrics struct holds metrics emitter configuration parameters.
type Metrics struct {
	Client   string    // The metrics client.
	Graphite *Graphite `mapstructure:"graphite"`
}

// Graphite struct holds attributes for the Graphite metrics emitter
type Graphite struct {
	Host          string        `mapstructure:"host"`
	Port          int           `mapstructure:"port"`
	Prefix        string        `mapstructure:"prefix"`
	FlushInterval time.Duration `mapstructure:"flushInterval"`
}

// CertSigner struct
type CertSigner struct {
	Client      string
	FakeSigner  *FakeSigner  `mapstructure:"fake"`
	LemurSigner *LemurSigner `mapstructure:"lemur"`
}

// FakeSigner struct holds SSL/TLS cert signing attributes.
type FakeSigner struct {
	Client     string   `mapstructure:"client"`
	Passphrase string   `mapstructure:"passphrase"`
	Bin        string   `mapstructure:"bin"`
	Args       []string `mapstructure:"args"`
}

// LemurSigner struct holds SSL/TLS cert signing attributes.
type LemurSigner struct {
	Client        string `mapstructure:"client"`
	Authority     string `mapstructure:"authority"`
	ValidityYears string `mapstructure:"validity_years"`
	Owner         string `mapstructure:"owner_email"`
	Key           string `mapstructure:"auth_token"`
	Bin           string `mapstructure:"bin"`
	Endpoint      string `mapstructure:"endpoint"`
}

// FilterParams struct holds various asset filter arguments that may be passed via cli args.
type FilterParams struct {
	Chassis bool
	Servers bool
	All     bool
	Serials string // Can be one or more serials separated by commas.
	Ips     string
}

// Vault struct declares vault config attributes
type Vault struct {
	TokenFromFile string `mapstructure:"tokenFromFile"`
	TokenFromEnv  bool   `mapstructure:"tokenFromEnv"`
	SecretsPath   string `mapstructure:"secretsPath"`
	HostAddress   string `mapstructure:"hostAddress"`
	Token         string `mapstructure:"token"`
}
