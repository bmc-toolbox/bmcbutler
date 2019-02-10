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

package cmd

import (
	"log"

	"github.com/bmc-toolbox/bmcbutler/pkg/app"
	"github.com/spf13/cobra"
)

// configureCmd represents the configure command
var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Apply config to bmcs.",
	Run: func(cmd *cobra.Command, args []string) {
		configure()
	},
}

func init() {
	rootCmd.AddCommand(configureCmd)
}

func validateConfigureArgs() {

	//one of these args are required
	if !runConfig.FilterParams.All &&
		!runConfig.FilterParams.Chassis &&
		!runConfig.FilterParams.Servers &&
		runConfig.FilterParams.Serials == "" &&
		runConfig.FilterParams.Ips == "" {

		log.Fatal("Expected flag missing --all/--chassis/--servers/--serials/--ips (try --help)")
	}

	if runConfig.FilterParams.All && (runConfig.FilterParams.Serials != "" || runConfig.FilterParams.Ips != "") {
		log.Fatal("--all --serial --ip are mutually exclusive args.")
	}

}

func configure() {

	runConfig.Configure = true
	validateConfigureArgs()

	options := app.Options{
		Configure:      true,
		ButlersToSpawn: butlersToSpawn,
		Locations:      locations,
		Resources:      resources,
		Tui:            runtui,
	}

	app := app.New(options, runConfig)

	app.Run()

}
