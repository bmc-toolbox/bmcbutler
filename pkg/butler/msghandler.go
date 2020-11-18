package butler

import (
	"github.com/sirupsen/logrus"

	metrics "github.com/bmc-toolbox/gin-go-metrics"
)

func (b *Butler) myLocation(location string) bool {
	for _, l := range b.Config.Locations {
		if l == location {
			return true
		}
	}

	return false
}

// msgHandler invokes the appropriate action based on msg attributes.
// nolint: gocyclo
func (b *Butler) msgHandler(msg Msg) {

	// if an interrupt was received, return.
	if b.interrupt {
		return
	}

	log := b.Log
	component := "msgHandler"

	metrics.IncrCounter([]string{"butler", "asset_recvd"}, 1)

	//if asset has no IPAddress, we can't do anything about it
	if len(msg.Asset.IPAddresses) == 0 {
		log.WithFields(logrus.Fields{
			"component": component,
			"Serial":    msg.Asset.Serial,
			"AssetType": msg.Asset.Type,
		}).Debug("Asset was received by butler without any IP(s) info, skipped.")

		metrics.IncrCounter([]string{"butler", "asset_recvd_noip"}, 1)
		return
	}

	//if asset has a location defined, we may want to filter it
	if msg.Asset.Location != "" {
		if !b.myLocation(msg.Asset.Location) && !b.Config.IgnoreLocation {
			log.WithFields(logrus.Fields{
				"component":     component,
				"Serial":        msg.Asset.Serial,
				"AssetType":     msg.Asset.Type,
				"AssetLocation": msg.Asset.Location,
			}).Warn("Butler wont manage asset based on its current location.")

			metrics.IncrCounter([]string{"butler", "asset_recvd_location_unmanaged"}, 1)
			return
		}
	}

	switch {
	case msg.Asset.Execute == true:
		err := b.executeCommand(msg.AssetExecute, &msg.Asset)
		if err != nil {
			log.WithFields(logrus.Fields{
				"component": component,
				"Serial":    msg.Asset.Serial,
				"AssetType": msg.Asset.Type,
				"Vendor":    msg.Asset.Vendor, //at this point the vendor may or may not be known.
				"Location":  msg.Asset.Location,
				"Error":     err,
			}).Warn("Unable Execute command(s) on asset.")
			metrics.IncrCounter([]string{"butler", "execute_fail"}, 1)
			return
		}

		metrics.IncrCounter([]string{"butler", "execute_success"}, 1)
		return
	case msg.Asset.Configure == true:
		err := b.configureAsset(msg.AssetConfig, &msg.Asset)
		if err != nil {
			log.WithFields(logrus.Fields{
				"component": component,
				"Serial":    msg.Asset.Serial,
				"AssetType": msg.Asset.Type,
				"Vendor":    msg.Asset.Vendor, //at this point the vendor may or may not be known.
				"Location":  msg.Asset.Location,
				"Error":     err,
			}).Warn("Configure action returned error.")

			metrics.IncrCounter([]string{"butler", "configure_fail"}, 1)
			return
		}

		metrics.IncrCounter([]string{"butler", "configure_success"}, 1)
		return
	default:
		log.WithFields(logrus.Fields{
			"component": component,
			"Serial":    msg.Asset.Serial,
			"AssetType": msg.Asset.Type,
			"Vendor":    msg.Asset.Vendor, //at this point the vendor may or may not be known.
			"Location":  msg.Asset.Location,
		}).Warn("Unknown action request on asset.")
	} //switch
}
