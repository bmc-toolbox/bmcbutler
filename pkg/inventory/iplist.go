package inventory

import (
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/config"
)

// An inventory source that holds attributes to setup the IP list source.
type IPList struct {
	Log       *logrus.Logger
	BatchSize int                  // Number of inventory assets to return per iteration.
	Channel   chan<- []asset.Asset // The channel to send inventory assets over.
	Config    *config.Params       // bmcbutler config + CLI params passed by the user.
}

func (i *IPList) AssetRetrieve() func() {
	return i.AssetIter
}

// AssetIter is an iterator method that sends assets to configure
// over the inventory channel.
func (i *IPList) AssetIter() {
	ips := strings.Split(i.Config.FilterParams.Ips, ",")

	assets := make([]asset.Asset, 0)
	for _, ip := range ips {
		assets = append(assets, asset.Asset{IPAddress: ip})
	}

	i.Channel <- assets
	close(i.Channel)
}
