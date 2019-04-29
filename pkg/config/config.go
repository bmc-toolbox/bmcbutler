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
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// Params struct holds all bmcbutler configuration parameters
type Params struct {
	ButlersToSpawn int                 `mapstructure:"butlersToSpawn"`
	Credentials    []map[string]string `mapstructure:"credentials"`
	CertSigner     *CertSigner         `mapstructure:"cert_signer"`
	Inventory      *Inventory          `mapstructure:"inventory"`
	Locations      []string            `mapstructure:"locations"`
	Metrics        *Metrics            `mapstructure:"metrics"`
	FilterParams   *FilterParams
	CfgFile        string
	Configure      bool //indicates configure was invoked
	DryRun         bool //when set, don't carry out any actions, just log.
	Execute        bool //indicates execute was invoked
	IgnoreLocation bool
	Resources      []string
	Version        string
	Debug          bool
	Trace          bool
}

// Inventory struct holds inventory configuration parameters.
type Inventory struct {
	Source string //dora, csv, enc
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
	Client   string    //The metrics client.
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
	Serials string //can be one or more serials separated by commas.
	Ips     string
}

// Load sets up bmcbutler configuration.
// nolint: gocyclo
func (p *Params) Load(cfgFile string) {

	//FilterParams holds the configure/setup/execute related host filter cli args.
	p.FilterParams = &FilterParams{}

	//read in config file with viper
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.SetConfigName("bmcbutler")
		viper.AddConfigPath("/etc/bmcbutler")
		viper.AddConfigPath(fmt.Sprintf("%s/.bmcbutler", home))
	}

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	err = viper.Unmarshal(&p)
	if err != nil {
		log.Fatal(err)
	}

	// metrics config
	if p.Metrics != nil {
		if p.Metrics.Graphite != nil {
			p.Metrics.Client = "graphite"
		} else {
			log.Println("[WARN] Invalid metrics client declared in config.")
		}
	}

	//signer config
	if p.CertSigner != nil {
		if p.CertSigner.FakeSigner != nil {
			p.CertSigner.Client = "fakeSigner"

		} else if p.CertSigner.LemurSigner != nil {
			p.CertSigner.Client = "lemurSigner"

		} else {
			log.Println("[WARN] Invalid cert_signer declared in config.")
		}
	}

	//inventory config
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

	//Butlers to spawn
	if p.ButlersToSpawn == 0 {
		p.ButlersToSpawn = 5
	}

	if p.Credentials == nil {
		log.Println("[Error] No credentials declared in configuration.")
		os.Exit(1)
	}
}
