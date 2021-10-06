package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/butler"
	"github.com/bmc-toolbox/bmcbutler/pkg/inventory"
	"github.com/bmc-toolbox/bmcbutler/pkg/secrets"
	metrics "github.com/bmc-toolbox/gin-go-metrics"
)

var (
	butlers   *butler.Butler
	commandWG sync.WaitGroup
	interrupt bool
)

// post handles clean up actions
// - closes the butler channel
// - Waits for all go routines in commandWG to finish.
func post(butlerChan chan butler.Msg) {
	close(butlerChan)
	commandWG.Wait()
	metrics.Close(true)
}

// Any flags to override configuration goes here.
func overrideConfigFromFlags() {
	if butlersToSpawn > 0 {
		runConfig.ButlersToSpawn = butlersToSpawn
	}

	if locations != "" {
		runConfig.Locations = strings.Split(locations, ",")
	}

	if resources != "" {
		runConfig.Resources = strings.Split(resources, ",")
	}

	runConfig.CfgFile = cfgFile

	if runConfig.DryRun {
		log.Info("Invoked with --dryrun.")
	}
}

// Sets up required plumbing and returns three channels.
// - Spawn a Go routine to listen to interrupt signals
// - Setup metrics channel
// - Spawn the metrics forwarder Go routine
// - Setup the inventory channel over which to receive assets
// - Based on the inventory source (dora/csv), spawn the asset retriever Go routine
// - Spawn butlers
// - Return inventory channel, butler channel
func prepareChannels() (inventoryChan chan []asset.Asset, butlerChan chan butler.Msg, stopChan chan struct{}) {
	overrideConfigFromFlags()
	runConfig.Load(runConfig.CfgFile)

	// Used to indicate Go routines to exit.
	stopChan = make(chan struct{})

	err := metrics.Setup(
		runConfig.Metrics.Client,
		runConfig.Metrics.Graphite.Host,
		runConfig.Metrics.Graphite.Port,
		runConfig.Metrics.Graphite.Prefix,
		runConfig.Metrics.Graphite.FlushInterval,
	)
	if err != nil {
		fmt.Printf("Failed to set up monitoring: %s", err)
		os.Exit(1)
	}

	// A channel to receive inventory assets.
	inventoryChan = make(chan []asset.Asset, 5)

	// Determine inventory to fetch asset data.
	inventorySource := runConfig.Inventory.Source

	// Based on inventory source, invoke assetRetriever:
	var assetRetriever func()

	switch inventorySource {
	case "enc":
		inventoryInstance := inventory.Enc{
			Config:     runConfig,
			Log:        log,
			BatchSize:  10,
			AssetsChan: inventoryChan,
			StopChan:   stopChan,
		}

		assetRetriever = inventoryInstance.AssetRetrieve()
	case "csv":
		inventoryInstance := inventory.Csv{
			Config:     runConfig,
			Log:        log,
			AssetsChan: inventoryChan,
		}

		assetRetriever = inventoryInstance.AssetRetrieve()
	case "dora":
		inventoryInstance := inventory.Dora{
			Config:     runConfig,
			Log:        log,
			BatchSize:  10,
			AssetsChan: inventoryChan,
		}

		assetRetriever = inventoryInstance.AssetRetrieve()
	case "iplist":
		inventoryInstance := inventory.IPList{
			Channel:   inventoryChan,
			Config:    runConfig,
			BatchSize: 1,
			Log:       log,
		}

		assetRetriever = inventoryInstance.AssetRetrieve()
	default:
		fmt.Println("Unknown/no inventory source declared in cfg: ", inventorySource)
		os.Exit(1)
	}

	// This routine returns assets over the inventoryChan.
	go assetRetriever()

	// Spawn butlers to work
	butlerChan = make(chan butler.Msg, 2)

	butlers = &butler.Butler{
		ButlerChan: butlerChan,
		StopChan:   stopChan,
		Config:     runConfig,
		Log:        log,
		SyncWG:     &commandWG,
	}

	if runConfig.SecretsFromVault {
		store, err := secrets.Load(*runConfig.Vault)
		if err != nil {
			log.Fatalf("[Error] loading secrets from vault: %s", err.Error())
		}

		runConfig.Credentials, err = store.SetCredentials(runConfig.Credentials)
		if err != nil {
			log.Fatalf("[Error] loading secrets from vault: %s", err.Error())
		}

		runConfig.CertSigner.LemurSigner.Key, err = store.GetSignerToken(runConfig.CertSigner.LemurSigner.Key)
		if err != nil {
			log.Fatalf("[Error] loading secrets from vault: %s", err.Error())
		}

		butlers.Secrets = store
	}

	go butlers.Runner()
	commandWG.Add(1)

	signalsChan := make(chan os.Signal, 1)
	signal.Notify(signalsChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-signalsChan:
			interrupt = true
			log.Warn("Interrupt SIGINT/SIGTERM received.")
			close(stopChan)
		case <-stopChan:
			return
		}
	}()

	return inventoryChan, butlerChan, stopChan
}
