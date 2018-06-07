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

package resource

import (
	"fmt"
	"github.com/bmc-toolbox/bmclib/cfgresources"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type Resource struct {
	Log *logrus.Logger
}

func (r *Resource) ReadResourcesConfig() (config *cfgresources.ResourcesConfig) {
	// returns a slice of configuration resources,
	// configuration resources may be applied periodically

	component := "resource"
	log := r.Log

	cfgDir := viper.GetString("bmcCfgDir")
	cfgFile := fmt.Sprintf("%s/%s", cfgDir, "configuration.yml")

	_, err := os.Stat(cfgFile)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"cfgFile":   cfgFile,
			"error":     err,
		}).Fatal("Declared cfg file not found.")
	}

	yamlData, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"cfgFile":   cfgFile,
			"error":     err,
		}).Fatal("Unable to read bmc cfg yaml.")
	}

	//1. read in data from common.yaml
	err = yaml.Unmarshal([]byte(yamlData), &config)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"cfgFile":   cfgFile,
			"error":     err,
		}).Fatal("Unable to Unmarshal common.yml.")
	}

	//read in data from vendor directories,
	//update config

	//read in data from dc directories

	//read in data from environment directories,
	return config
}

func (r *Resource) ReadResourcesSetup() (config *cfgresources.ResourcesSetup) {
	// returns a slice of setup resources to be applied,
	// 'setup' is config that is applied just once,
	// it may involve resetting/power cycling various dependencies,
	//  - e.g blades in a chassis that need to be power cycled
	//    if the flex addresses have been enabled/disabled.

	component := "resource"
	log := r.Log

	cfgDir := viper.GetString("bmcCfgDir")
	cfgFile := fmt.Sprintf("%s/%s", cfgDir, "setup.yml")

	_, err := os.Stat(cfgFile)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"cfgFile":   cfgFile,
			"error":     err,
		}).Fatal("Declared cfg file not found.")
	}

	yamlData, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"cfgFile":   cfgFile,
			"error":     err,
		}).Fatal("Unable to read bmc cfg yaml.")
	}

	//1. read in data from common.yaml
	err = yaml.Unmarshal([]byte(yamlData), &config)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"cfgFile":   cfgFile,
			"error":     err,
		}).Fatal("Unable to Unmarshal common.yml.")
	}
	return config
}
