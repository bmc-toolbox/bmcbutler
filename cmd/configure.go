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
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/butler"
	"github.com/bmc-toolbox/bmcbutler/pkg/inventory"
	"github.com/bmc-toolbox/bmcbutler/pkg/metrics"
	"github.com/bmc-toolbox/bmcbutler/pkg/resource"
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

	initConfig()

	//configure command flags
	configureCmd.Flags().BoolVarP(&runCfg.FilterParams.Chassis, "chassis", "", false, "Configure chassis asset(s).")
	configureCmd.Flags().BoolVarP(&runCfg.FilterParams.Blade, "blade", "", false, "Configure blade asset(s).")
	configureCmd.Flags().BoolVarP(&runCfg.FilterParams.Discrete, "discrete", "", false, "Configure discrete(s).")
	configureCmd.Flags().BoolVarP(&runCfg.FilterParams.All, "all", "", false, "Configure all assets.")
	configureCmd.Flags().StringVarP(&runCfg.FilterParams.Serials, "serial", "", "", "Configure one or more assets listed by serial(s) - use in conjunction with (--chassis/blade/discrete).")
	configureCmd.Flags().StringVarP(&runCfg.FilterParams.IpList, "iplist", "", "", "Configure one or more assets listed by IP address(es) - use in conjuction with (--chassis/blade/discrete).")
}

func validateConfigureArgs() {

	fmt.Printf("%+v", runCfg.FilterParams)
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

func configure() {

	validateConfigureArgs()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	//flag when its time to exit.
	var exitFlag bool

	go func() {
		_ = <-sigChan
		exitFlag = true
	}()

	//A sync waitgroup for routines spawned here.
	var configureWG sync.WaitGroup

	// A channel butlers sends metrics to the metrics sender
	metricsChan := make(chan []metrics.MetricsMsg, 5)

	//the metrics forwarder routine
	metricsForwarder := metrics.Metrics{
		Logger:  log,
		Channel: metricsChan,
		SyncWG:  &configureWG,
		Config:  &runCfg,
	}

	//metrics emitter instance, used by methods to emit metrics to the forwarder.
	metricsEmitter := metrics.Emitter{Channel: metricsChan}

	//spawn metrics forwarder routine
	go metricsForwarder.Run()
	configureWG.Add(1)

	// A channel to recieve inventory assets
	inventoryChan := make(chan []asset.Asset, 5)

	butlersToSpawn := runCfg.ButlersToSpawn
	inventorySource := runCfg.InventorySource

	//if --iplist was passed, set inventorySource
	if runCfg.FilterParams.IpList != "" {
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
		// Spawn a goroutine that returns a slice of assets over inventoryChan
		// the number of assets in the slice is determined by the batch size.

		inventoryInstance := inventory.Dora{
			Log:             log,
			BatchSize:       10,
			AssetsChan:      inventoryChan,
			MetricsEmitter:  metricsEmitter,
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
		go inventoryInstance.AssetIter(runCfg.FilterParams.IpList)

	default:
		fmt.Println("Unknown/no inventory source declared in cfg: ", inventorySource)
		os.Exit(1)
	}

	// Spawn butlers to work
	butlerChan := make(chan butler.ButlerMsg, 5)
	butlerManager := butler.ButlerManager{
		Log:            log,
		SpawnCount:     butlersToSpawn,
		ButlerChan:     butlerChan,
		MetricsEmitter: metricsEmitter,
	}

	if serial != "" {
		butlerManager.IgnoreLocation = true
	}

	go butlerManager.SpawnButlers()

	//give the butlers a second to spawn.
	time.Sleep(1 * time.Second)

	//Read in BMC configuration data
	configDir := viper.GetString("bmcCfgDir")
	configFile := fmt.Sprintf("%s/%s", configDir, "configuration.yml")

	//returns the file read as a slice of bytes
	//config may contain templated values.
	config, err := resource.ReadYamlTemplate(configFile)
	if err != nil {
		log.Fatal("Unable to read BMC configuration: ", configFile, " Error: ", err)
		os.Exit(1)
	}

	//iterate over the inventory channel for assets,
	//create a butler message for each asset along with the configuration,
	//at this point templated values in the config are not yet rendered.
	for assetList := range inventoryChan {
		for _, asset := range assetList {

			//if signal was received, break out.
			if exitFlag {
				break
			}

			asset.Configure = true

			//NOTE: if all butlers exit, and we're trying to write to butlerChan
			//      this loop is going to be stuck waiting for the butlerMsg to be read,
			//      make sure to break out of this loop or have butlerChan closed in such a case,
			//      for now, we fix this by setting exitFlag to break out of the loop.
			butlerMsg := butler.ButlerMsg{Asset: asset, Config: config}
			butlerChan <- butlerMsg
		}

		//if sigterm is received, break out.
		if exitFlag {
			break
		}
	}

	close(butlerChan)

	//wait until butlers are done.
	butlerManager.Wait()
	log.Debug("All butlers have exited.")

	close(metricsChan)
	configureWG.Wait()

}
