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
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/butler"
	"github.com/bmc-toolbox/bmcbutler/pkg/inventory"
)

// executeCmd represents the execute command
var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute actions on bmcs.",
	Run: func(cmd *cobra.Command, args []string) {
		execute()
	},
}

func init() {

	//load config
	initConfig()

	//execute command flags
	executeCmd.Flags().BoolVarP(&runCfg.FilterParams.Chassis, "chassis", "", false, "Execute command on chassis asset(s).")
	executeCmd.Flags().BoolVarP(&runCfg.FilterParams.Blade, "blade", "", false, "Executure command on blade asset(s).")
	executeCmd.Flags().BoolVarP(&runCfg.FilterParams.Discrete, "discrete", "", false, "Execute command on discrete(s).")
	executeCmd.Flags().BoolVarP(&runCfg.FilterParams.All, "all", "", false, "Execute on all assets.")
	executeCmd.Flags().StringVarP(&runCfg.FilterParams.Serials, "serial", "", "", "Execute command on one or more assets listed by serial(s) - use in conjunction with (--chassis/blade/discrete).")
	executeCmd.Flags().StringVarP(&runCfg.FilterParams.IpList, "iplist", "", "", "Execute command one or more assets listed by IP address(es) - use in conjuction with (--chassis/blade/discrete).")

}

func validateExecuteArgs() {

	//runCfg is declared in root.go and initialized in initConfig()
	if runCfg.FilterParams.All == true {
		assetType = "all"
		return
	}

	if runCfg.FilterParams.Chassis == false &&
		runCfg.FilterParams.Blade == false &&
		runCfg.FilterParams.Discrete == false {

		log.Error("Either --all OR --chassis OR --blade OR --discrete expected.")

		os.Exit(1)
	}

	if runCfg.FilterParams.Chassis == true {
		assetType = "chassis"
	} else if runCfg.FilterParams.Blade == true {
		assetType = "blade"
	} else if runCfg.FilterParams.Discrete == true {
		assetType = "discrete"
	}
}

func execute() {

	validateConfigureArgs()

	// A channel to recieve inventory assets
	inventoryChan := make(chan []asset.Asset, 5)

	butlersToSpawn := viper.GetInt("butlersToSpawn")

	if butlersToSpawn == 0 {
		butlersToSpawn = 5
	}

	inventorySource := viper.GetString("inventory.configure.source")

	//if --iplist was passed, set inventorySource
	if ipList != "" {
		inventorySource = "iplist"
	}

	switch inventorySource {
	case "csv":
		inventoryInstance := inventory.Csv{Log: log, Channel: inventoryChan}
		if all {
			go inventoryInstance.AssetIter()
		} else {
			go inventoryInstance.AssetIterBySerial(serial)
		}
	case "dora":
		inventoryInstance := inventory.Dora{
			Log:             log,
			BatchSize:       10,
			AssetsChan:      inventoryChan,
			FilterAssetType: assetType,
			FilterParams:    runCfg.FilterParams,
		}

		//assetRetriever is a function that retrieves assets
		var assetRetriever func()

		//based on FilterParams get a asset retriever
		assetRetriever = inventoryInstance.AssetRetrieve()
		go assetRetriever()

	case "iplist":
		inventoryInstance := inventory.IpList{Log: log, BatchSize: 1, Channel: inventoryChan}

		// invoke goroutine that passes assets by IP to spawned butlers,
		// here we declare setup = false since this is a configure action.
		go inventoryInstance.AssetIter(ipList)

	default:
		fmt.Println("Unknown/no inventory source declared in cfg: ", inventorySource)
		os.Exit(1)
	}

	// Spawn butlers to work
	butlerChan := make(chan butler.ButlerMsg, 5)
	butlerManager := butler.ButlerManager{Log: log, SpawnCount: butlersToSpawn, ButlerChan: butlerChan}

	if serial != "" {
		butlerManager.IgnoreLocation = true
	}

	go butlerManager.SpawnButlers()

	//give the butlers a second to spawn.
	time.Sleep(1 * time.Second)

	//iterate over the inventory channel for assets,
	//create a butler message for each asset along with the configuration,
	//at this point templated values in the config are not yet rendered.
	for assetList := range inventoryChan {
		for _, asset := range assetList {
			asset.Execute = true
			butlerMsg := butler.ButlerMsg{Asset: asset, Execute: execCommand}
			butlerChan <- butlerMsg
		}
	}

	close(butlerChan)

	//wait until butlers are done.
	butlerManager.Wait()
}
