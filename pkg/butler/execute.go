package butler

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/bmc-toolbox/bmclib/devices"
	"github.com/bmc-toolbox/bmclogin"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
)

// applyConfig setups up the bmc connection
// gets any config templated data rendered
// applies the configuration using bmclib
func (b *Butler) executeCommand(command string, asset *asset.Asset) (err error) {
	component := "executeCommand"
	log := b.Log

	if b.Config.DryRun {
		log.WithFields(logrus.Fields{
			"component": component,
		}).Info("Dry run, won't execute cmd on asset.")
		return nil
	}

	bmcConn := bmclogin.Params{
		IpAddresses:     asset.IPAddresses,
		Credentials:     b.Config.Credentials,
		CheckCredential: false,
		Retries:         1,
	}

	client, loginInfo, err := bmcConn.Login()
	if err != nil {
		return err
	}

	asset.IPAddress = loginInfo.ActiveIpAddress

	switch client.(type) {
	case devices.Bmc:
		bmc := client.(devices.Bmc)
		success, output, err := b.executeCommandBmc(bmc, command)
		if err != nil || !success {
			log.WithFields(logrus.Fields{
				"component":          component,
				"Serial":             asset.Serial,
				"Asset Type":         asset.Type,
				"Vendor":             asset.Vendor, // At this point, the vendor may or may not be known.
				"Location":           asset.Location,
				"IP Address":         asset.IPAddress,
				"Command":            command,
				"Command Successful": success,
				"Error":              err,
				"Output":             output,
			}).Warn("Command execute returned error.")
		} else {
			log.WithFields(logrus.Fields{
				"component":          component,
				"Serial":             asset.Serial,
				"Asset Type":         asset.Type,
				"Vendor":             asset.Vendor,
				"Location":           asset.Location,
				"IP Address":         asset.IPAddress,
				"Command":            command,
				"Command Successful": success,
				"Output":             output,
			}).Debug("Command successfully executed.")
		}
		bmc.Close(context.TODO())
	case devices.Cmc:
		chassis := client.(devices.Cmc)
		//b.executeCommandChassis(chassis, command)
		log.WithFields(logrus.Fields{
			"component": component,
		}).Info("Command executed.")
		chassis.Close()
	default:
		log.WithFields(logrus.Fields{
			"component": component,
		}).Warn("Unknown device type.")
		return errors.New("unknown asset type")
	}

	return err
}

func (b *Butler) executeCommandBmc(bmc devices.Bmc, command string) (success bool, output string, err error) {
	switch command {
	case "bmc-reset":
		success, err := bmc.PowerCycleBmc()
		return success, "", err
	case "powercycle":
		success, err := bmc.PowerCycle()
		return success, "", err
	case "firmware-update":
		class, err := bmc.Class()
		if err != nil {
			return false, "", err
		}

		success, output, err := bmc.UpdateFirmware("10.198.174.2", "bmc-firmware/"+bmc.Vendor()+"/"+class)
		return success, output, err
	case "firmware-version":
		output, err := bmc.CheckFirmwareVersion()
		return err == nil, output, err
	default:
		return success, "", fmt.Errorf("unknown command: %s", command)
	}
}

//func (b *Butler) executeCommandChassis(chassis devices.Cmc, command []byte) (err error) {
//
//	switch string(command) {
//	case "Chassis reset":
//		chassis.PowerCycleBmc()
//	default:
//		return errors.New(fmt.Sprintf("Unknown command: %s", command))
//	}
//
//	return err
//}
