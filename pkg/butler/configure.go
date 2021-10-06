package butler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/butler/configure"
	"github.com/bmc-toolbox/bmcbutler/pkg/resource"
	"github.com/bmc-toolbox/bmclib/devices"
	"github.com/bmc-toolbox/bmclogin"
	metrics "github.com/bmc-toolbox/gin-go-metrics"
)

// applyConfig setups up the bmc connection
// gets any Asset config templated data rendered
// applies the asset configuration using bmclib
func (b *Butler) configureAsset(config []byte, asset *asset.Asset) (err error) {

	log := b.Log
	component := "configureAsset"

	if b.Config.DryRun {
		log.WithFields(logrus.Fields{
			"component": component,
			"Asset":     fmt.Sprintf("%+v", asset),
		}).Info("Dry run, asset configuration will be skipped.")
		return nil
	}

	defer metrics.MeasureRuntime([]string{"butler", "configure_runtime"}, time.Now())

	b.Log.WithFields(logrus.Fields{
		"component": component,
		"Serial":    asset.Serial,
		"IPAddress": asset.IPAddresses,
	}).Debug("Connecting to asset.")

	bmcConn := bmclogin.Params{
		IpAddresses:     asset.IPAddresses,
		Credentials:     b.Config.Credentials,
		CheckCredential: true,
		Retries:         1,
		StopChan:        b.StopChan,
	}

	//connect to the bmc/chassis bmc
	client, loginInfo, err := bmcConn.Login()
	if err != nil {
		return err
	}

	asset.IPAddress = loginInfo.ActiveIpAddress

	switch client.(type) {
	case devices.Bmc:

		bmc := client.(devices.Bmc)

		asset.Type = "server"
		asset.HardwareType = bmc.HardwareType()
		asset.Vendor = bmc.Vendor()
		// Required for TLS cert CN
		asset.Serial, _ = bmc.Serial()

		//Setup a resource instance
		//Get any templated values in the asset config rendered
		resourceInstance := resource.Resource{Log: log, Asset: asset, Secrets: b.Secrets}

		//rendered config is a *cfgresources.ResourcesConfig type
		renderedConfig := resourceInstance.LoadConfigResources(config)
		if renderedConfig == nil {
			return errors.New("No BMC configuration to be applied")
		}

		// Apply configuration
		c := configure.NewBmcConfigurator(bmc, asset, b.Config.Resources, renderedConfig, b.Config, b.StopChan, log)
		c.Apply()

		bmc.Close(context.TODO())
	case devices.Cmc:
		chassis := client.(devices.Cmc)

		asset.Type = "chassis"
		asset.HardwareType = chassis.HardwareType()
		asset.Vendor = chassis.Vendor()

		// Required for TLS cert CN
		asset.Serial, _ = chassis.Serial()

		//Setup a resource instance
		//Get any templated values in the asset config rendered
		resourceInstance := resource.Resource{Log: log, Asset: asset, Secrets: b.Secrets}

		renderedConfig := resourceInstance.LoadConfigResources(config)
		if renderedConfig == nil {
			return errors.New("No BMC configuration to be applied")
		}

		if renderedConfig.SetupChassis != nil {
			s := configure.NewCmcSetup(
				chassis,
				asset,
				b.Config.Resources,
				renderedConfig.SetupChassis,
				b.Config,
				b.StopChan,
				b.Log,
			)
			s.Apply()
		}

		// Apply configuration
		c := configure.NewCmcConfigurator(chassis, asset, b.Config.Resources, renderedConfig, b.StopChan, log)
		c.Apply()

		chassis.Close()
	default:
		log.WithFields(logrus.Fields{
			"component": component,
			"Asset":     fmt.Sprintf("%+v", asset),
		}).Warn("Unknown device type.")
		return errors.New("Unknown asset type")
	}

	return err
}
