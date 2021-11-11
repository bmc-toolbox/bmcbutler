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
	component := "configureAsset"

	if b.Config.DryRun {
		b.Log.WithFields(logrus.Fields{
			"component": component,
		}).Info("Dry run, asset configuration will be skipped.")
		return nil
	}

	defer b.timeTrack(time.Now(), "configureAsset", asset)
	defer metrics.MeasureRuntime([]string{"butler", "configure_runtime"}, time.Now())

	b.Log.WithFields(logrus.Fields{
		"component": component,
		"Serial":    asset.Serial,
		"IPAddress": asset.IPAddresses,
	}).Debug("Connecting to asset...")

	bmcConn := bmclogin.Params{
		IpAddresses:     asset.IPAddresses,
		Credentials:     b.Config.Credentials,
		CheckCredential: true,
		Retries:         1,
		StopChan:        b.StopChan,
	}

	client, loginInfo, err := bmcConn.Login()
	if err != nil {
		return err
	}

	asset.IPAddress = loginInfo.ActiveIpAddress

	switch clientType := client.(type) {
	case devices.Bmc:
		bmc := client.(devices.Bmc)

		asset.Type = "server"
		asset.HardwareType = bmc.HardwareType()
		asset.Vendor = bmc.Vendor()

		// We already have the asset serial from the inventory source.
		// This is done for sanity checking. Sometimes a device's serial changes because
		//   of a motherboard change, however. It's a valid case but should be rare.
		s, err := bmc.Serial()
		if err != nil {
			b.Log.WithFields(logrus.Fields{
				"component":       component,
				"InventorySerial": asset.Serial,
			}).Warn("Error getting BMC serial!")
		} else if asset.Serial != s {
			b.Log.WithFields(logrus.Fields{
				"component":       component,
				"BMCSerial":       s,
				"InventorySerial": asset.Serial,
			}).Warn("The BMC reports a different serial than the inventory source!")
		}

		// Gets any templated values in the asset configuration rendered.
		resourceInstance := resource.Resource{Log: b.Log, Asset: asset, Secrets: b.Secrets}
		renderedConfig := resourceInstance.LoadConfigResources(config)
		if renderedConfig == nil {
			return errors.New("No BMC configuration to be applied!")
		}

		c := configure.NewBmcConfigurator(bmc, asset, b.Config.Resources, renderedConfig, b.Config, b.StopChan, b.Log)
		c.Apply()

		bmc.Close(context.TODO())
	case devices.Cmc:
		chassis := client.(devices.Cmc)

		asset.Type = "chassis"
		asset.HardwareType = chassis.HardwareType()
		asset.Vendor = chassis.Vendor()

		// We already have the asset serial from the inventory source.
		// This is done for sanity checking. Sometimes a device's serial changes because
		//   of a motherboard change, however. It's a valid case but should be rare.
		s, err := chassis.Serial()
		if err != nil {
			b.Log.WithFields(logrus.Fields{
				"component":       component,
				"InventorySerial": asset.Serial,
			}).Warn("Error getting CMC serial!")
		} else if asset.Serial != s {
			b.Log.WithFields(logrus.Fields{
				"component":       component,
				"CMCSerial":       s,
				"InventorySerial": asset.Serial,
			}).Warn("The CMC reports a different serial than the inventory source!")
		}

		resourceInstance := resource.Resource{Log: b.Log, Asset: asset, Secrets: b.Secrets}
		renderedConfig := resourceInstance.LoadConfigResources(config)
		if renderedConfig == nil {
			return errors.New("No CMC configuration to be applied!")
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
		c := configure.NewCmcConfigurator(chassis, asset, b.Config.Resources, renderedConfig, b.StopChan, b.Log)
		c.Apply()

		chassis.Close()
	default:
		b.Log.WithFields(logrus.Fields{
			"component": component,
			"Type":      fmt.Sprintf("%s", clientType),
		}).Warn("Unknown device type.")
		return fmt.Errorf("Unknown device type \"%s\"!", clientType)
	}

	return err
}
