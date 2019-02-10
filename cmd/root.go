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
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bmc-toolbox/bmcbutler/pkg/config"
)

var (
	butlersToSpawn int
	cfgFile        string
	execCommand    string
	locations      string
	resources      string
	runConfig      *config.Params
	runtui         bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:              "bmcbutler",
	Short:            "A bmc config manager",
	TraverseChildren: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {

	//bmcbutler runtime configuration.
	runConfig = &config.Params{}
	runConfig.Load(cfgFile)

	rootCmd.PersistentFlags().BoolVarP(&runConfig.Debug, "debug", "d", false, "debug logging")
	rootCmd.PersistentFlags().BoolVarP(&runConfig.Trace, "trace", "t", false, "trace logging")
	rootCmd.PersistentFlags().BoolVarP(&runtui, "tui", "", false, "run tui")

	//Asset filter params.
	rootCmd.PersistentFlags().BoolVarP(&runConfig.FilterParams.All, "all", "", false, "Action all assets")
	rootCmd.PersistentFlags().BoolVarP(&runConfig.FilterParams.Chassis, "chassis", "", false, "Action just Chassis assets.")
	rootCmd.PersistentFlags().BoolVarP(&runConfig.FilterParams.Servers, "servers", "", false, "Action just Server assets.")
	rootCmd.PersistentFlags().BoolVarP(&runConfig.DryRun, "dryrun", "", false, "Only log assets that will be actioned.")
	rootCmd.PersistentFlags().StringVarP(&runConfig.FilterParams.Serials, "serials", "", "", "Serial(s) of the asset to setup config (separated by commas - no spaces).")
	rootCmd.PersistentFlags().StringVarP(&runConfig.FilterParams.Ips, "ips", "", "", "IP Address(s) of the asset to setup config (separated by commas - no spaces).")

	rootCmd.PersistentFlags().BoolVarP(&runConfig.IgnoreLocation, "ignorelocation", "", false, "Action assets in all locations (ignore locations directive in config)")
	rootCmd.PersistentFlags().IntVarP(&butlersToSpawn, "butlers", "b", 0, "Number of butlers to spawn (overide butlersToSpawn directive in config)")
	rootCmd.PersistentFlags().StringVarP(&locations, "locations", "l", "", "Action assets by given location(s). (overide locations directive in config)")
	rootCmd.PersistentFlags().StringVarP(&resources, "resources", "r", "", "Apply one or more resources instead of the whole config (e.g -r syslog,ntp).")
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Configuration file for bmcbutler (default: /etc/bmcbutler/bmcbutler.yml)")

	//move to exec
	rootCmd.PersistentFlags().StringVarP(&execCommand, "command", "", "", "Command to execute on BMCs.")

	//NOTE: to override any config from the flags declared here, see overrideConfigFromFlags in common.go
}
