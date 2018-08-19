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
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

type Params struct {
	Version         string
	Verbose         bool
	CfgFile         string
	InventorySource string
	IgnoreLocation  bool
	ButlersToSpawn  int
	FilterParams    *FilterParams
	Metrics         *Metrics
}

type FilterParams struct {
	Chassis  bool
	Blade    bool
	Discrete bool
	All      bool
	Serials  string
	IpList   string
}

type Metrics struct {
	Target         string
	GraphiteHost   string
	GraphitePort   int
	GraphitePrefix string
}

//Config params constructor
func (p *Params) Load(cfgFile string) {

	p.FilterParams = &FilterParams{}
	p.Metrics = &Metrics{}

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

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Error reading config:", viper.ConfigFileUsed())
		fmt.Println("  ->", err)
		os.Exit(1)
	}

	//The config file viper is using.
	p.CfgFile = viper.ConfigFileUsed()

	//Read in metrics config
	p.Metrics.Target = viper.GetString("metrics.receiver.target")
	switch p.Metrics.Target {
	case "graphite":
		//TODO: add validation
		p.Metrics.GraphiteHost = viper.GetString("metrics.receiver.graphite.host")
		p.Metrics.GraphitePort = viper.GetInt("metrics.receiver.graphite.port")
		p.Metrics.GraphitePrefix = viper.GetString("metrics.receiver.graphite.prefix")
	}

	//Butlers to spawn
	p.ButlersToSpawn = viper.GetInt("butlersToSpawn")
	if p.ButlersToSpawn == 0 {
		p.ButlersToSpawn = 5
	}

	//Inventory to read assets from
	p.InventorySource = viper.GetString("inventory.configure.source")

}
