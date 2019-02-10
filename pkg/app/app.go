package app

import (
	"fmt"
	"log/syslog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	logrusSyslog "github.com/sirupsen/logrus/hooks/syslog"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/butler"
	"github.com/bmc-toolbox/bmcbutler/pkg/config"
	"github.com/bmc-toolbox/bmcbutler/pkg/inventory"
	"github.com/bmc-toolbox/bmcbutler/pkg/metrics"
	"github.com/bmc-toolbox/bmcbutler/pkg/resource"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// App sets up the application to run
type App struct {
	config    *config.Params
	waitGroup sync.WaitGroup
	// SIGTERM/INT received
	interrupt      bool
	log            *logrus.Logger
	metricsEmitter *metrics.Emitter
	// on close notify goroutines to return
	stopChan chan struct{}
	Options
}

// Options are App options
type Options struct {
	Configure bool
	Execute   bool
	// Number of butler workers to spawn
	ButlersToSpawn int
	Locations      string
	// Just apply specific resources, instead of the whole config template
	Resources string
	Tui       bool
	DryRun    bool
}

// New returns a new Application
func New(options Options, config *config.Params) *App {
	return &App{
		config:  config,
		Options: options,
	}
}

// Run runs the application.
func (a *App) Run() {

	a.setupLogger()

	inventoryChan, butlerChan, stopChan := a.pre()

	//Read in BMC configuration data
	assetConfigDir := viper.GetString("bmcCfgDir")
	assetConfigFile := fmt.Sprintf("%s/%s", assetConfigDir, "configuration.yml")

	//returns the file read as a slice of bytes
	//config may contain templated values.
	assetConfig, err := resource.ReadYamlTemplate(assetConfigFile)
	if err != nil {
		a.log.Fatal("Unable to read BMC configuration: ", assetConfigFile, " Error: ", err)
		os.Exit(1)
	}

	//iterate over the inventory channel for assets,
	//create a butler message for each asset along with the configuration,
	//at this point templated values in the config are not yet rendered.
loop:
	for {
		select {
		case assetList, ok := <-inventoryChan:
			if !ok {
				break loop
			}
			for _, asset := range assetList {
				if a.Options.Configure {
					asset.Configure = true
				}

				if a.Options.Execute {
					asset.Execute = true
				}

				butlerMsg := butler.Msg{Asset: asset, AssetConfig: assetConfig}
				if a.interrupt {
					break loop
				}

				butlerChan <- butlerMsg
			}
		case <-stopChan:
			a.interrupt = true
		}
	}

	a.post(butlerChan)

}

func (a *App) setupLogger() {

	//setup logging
	a.log = logrus.New()
	a.log.Out = os.Stdout

	hook, err := logrusSyslog.NewSyslogHook("", "", syslog.LOG_INFO, "BMCbutler")
	if err != nil {
		a.log.Error("Unable to connect to local syslog daemon.")
	} else {
		a.log.AddHook(hook)
	}

	switch {
	case a.config.Debug == true:
		a.log.SetLevel(logrus.DebugLevel)
	case a.config.Trace == true:
		a.log.SetLevel(logrus.TraceLevel)
	default:
		a.log.SetLevel(logrus.InfoLevel)
	}
}

// Any flags to override configuration goes here.
func (a *App) overrideConfigFromFlags() {
	if a.ButlersToSpawn > 0 {
		a.config.ButlersToSpawn = a.ButlersToSpawn
	}

	if a.Locations != "" {
		a.config.Locations = strings.Split(a.Locations, ",")
	}

	if a.Resources != "" {
		a.config.Resources = strings.Split(a.Resources, ",")
	}

	if a.DryRun {
		a.log.Info("Invoked with --dryrun.")
	}
}

// post handles clean up actions
// - closes the butler channel
// - Waits for all go routines in waitGroup to finish.
func (a *App) post(butlerChan chan butler.Msg) {
	close(butlerChan)
	a.waitGroup.Wait()
	a.metricsEmitter.Close(true)
}

// pre sets up required plumbing and returns two channels.
// - Spawn go routine to listen to interrupt signals
// - Setup metrics channel
// - Spawn the metrics forwarder go routine
// - Setup the inventory channel over which to receive assets
// - Based on the inventory source (dora/csv), Spawn the asset retriever go routine.
// - Spawn butlers
// - Return inventory channel, butler channel.
func (a *App) pre() (inventoryChan chan []asset.Asset, butlerChan chan butler.Msg, stopChan chan struct{}) {

	a.overrideConfigFromFlags()

	//Channel used to indicate goroutines to exit.

	stopChan = make(chan struct{})

	//Initialize metrics collection.
	a.metricsEmitter = &metrics.Emitter{
		Config: a.config,
		Logger: a.log,
	}

	a.metricsEmitter.Init()

	// A channel to receive inventory assets
	inventoryChan = make(chan []asset.Asset, 5)

	//determine inventory to fetch asset data.
	inventorySource := a.config.InventoryParams.Source

	// run terminal user interface
	//if runtui {
	//	ui, err := tui.NewUserInterface(stopChan, &waitGroup, a.log)
	//	if err != nil {
	//		fmt.Printf("tui setup error: %s", err)
	//		os.Exit(1)
	//	}

	//	err = ui.Run()
	//	if err != nil {
	//		fmt.Printf("tui run error: %s", err)
	//		os.Exit(1)
	//	}
	//}

	//based on inventory source, invoke assetRetriever
	var assetRetriever func()

	switch inventorySource {
	case "enc":
		inventoryInstance := inventory.Enc{
			Config:         a.config,
			Log:            a.log,
			BatchSize:      10,
			AssetsChan:     inventoryChan,
			MetricsEmitter: a.metricsEmitter,
			StopChan:       stopChan,
		}

		assetRetriever = inventoryInstance.AssetRetrieve()
	case "csv":
		inventoryInstance := inventory.Csv{
			Config:     a.config,
			Log:        a.log,
			AssetsChan: inventoryChan,
		}

		assetRetriever = inventoryInstance.AssetRetrieve()
	case "dora":
		inventoryInstance := inventory.Dora{
			Config:         a.config,
			Log:            a.log,
			BatchSize:      10,
			AssetsChan:     inventoryChan,
			MetricsEmitter: a.metricsEmitter,
		}

		assetRetriever = inventoryInstance.AssetRetrieve()
	case "iplist":
		inventoryInstance := inventory.IPList{
			Channel:   inventoryChan,
			Config:    a.config,
			BatchSize: 1,
			Log:       a.log,
		}

		assetRetriever = inventoryInstance.AssetRetrieve()
	default:
		fmt.Println("Unknown/no inventory source declared in cfg: ", inventorySource)
		os.Exit(1)
	}

	//invoke asset retriever routine
	//this routine returns assets over the inventoryChan.
	go assetRetriever()

	// Spawn butlers to work
	butlerChan = make(chan butler.Msg, 2)
	butlers := &butler.Butler{
		ButlerChan:     butlerChan,
		StopChan:       stopChan,
		Config:         a.config,
		Log:            a.log,
		MetricsEmitter: a.metricsEmitter,
		SyncWG:         &a.waitGroup,
	}

	go butlers.Runner()
	a.waitGroup.Add(1)

	//setup a sigchan
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			a.interrupt = true
			a.log.Warn("Interrupt SIGINT/SIGTERM received.")
			close(stopChan)
		case <-stopChan:
			return
		}
	}()

	return inventoryChan, butlerChan, stopChan
}
