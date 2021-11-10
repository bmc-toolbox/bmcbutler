package inventory

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/config"
	metrics "github.com/bmc-toolbox/gin-go-metrics"
	"github.com/sirupsen/logrus"
)

// Enc struct holds attributes required to run inventory/enc methods.
type Enc struct {
	Log             *logrus.Logger
	BatchSize       int
	AssetsChan      chan<- []asset.Asset
	Config          *config.Params
	FilterAssetType []string
	StopChan        <-chan struct{}
}

// AssetAttributes is used to unmarshal data returned from an ENC.
type AssetAttributes struct {
	Data        map[string]Attributes `json:"data"` // Map of asset IPs/Serials to attributes.
	EndOfAssets bool                  `json:"end_of_assets"`
}

// Attributes is used to unmarshal data returned from an ENC.
type Attributes struct {
	Location          string              `json:"location"`
	NetworkInterfaces *[]NetworkInterface `json:"network_interfaces"`
	BMCIPAddresses    []string            `json:"-"`
	Extras            *AttributesExtras   `json:"extras"`
}

// NetworkInterface is used to unmarshal data returned from the ENC.
type NetworkInterface struct {
	Name       string `json:"name"`
	MACAddress string `json:"mac_address"`
	IPAddress  string `json:"ip_address"`
}

// AttributesExtras is used to unmarshal data returned from an ENC.
type AttributesExtras struct {
	State   string `json:"status"`
	Company string `json:"company"`
	// If it's a chassis, this would hold serials for blades in the Live state.
	LiveAssets *[]string `json:"live_assets,omitempty"`
}

func stringHasPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			return true
		}
	}

	return false
}

// SetBMCInterfaces populates IPAddresses of BMC interfaces,
// from the Slice of NetworkInterfaces
func (e *Enc) SetBMCInterfaces(attributes Attributes) Attributes {
	if attributes.NetworkInterfaces == nil {
		return attributes
	}

	bmcNicPrefixes := e.Config.Inventory.Enc.BMCNicPrefix
	for _, nic := range *attributes.NetworkInterfaces {
		if stringHasPrefix(nic.Name, bmcNicPrefixes) && nic.IPAddress != "" && nic.IPAddress != "0.0.0.0" {
			attributes.BMCIPAddresses = append(attributes.BMCIPAddresses, nic.IPAddress)
		}
	}

	return attributes
}

// AttributesExtrasAsMap accepts a AttributesExtras struct as input,
// and returns all attributes as a map
func AttributesExtrasAsMap(attributeExtras *AttributesExtras) (extras map[string]string) {
	extras = make(map[string]string)

	extras["state"] = strings.ToLower(attributeExtras.State)
	extras["company"] = strings.ToLower(attributeExtras.Company)

	if attributeExtras.LiveAssets != nil {
		extras["liveAssets"] = strings.ToLower(strings.Join(*attributeExtras.LiveAssets, ","))
	} else {
		extras["liveAssets"] = ""
	}

	return extras
}

func (e *Enc) AssetRetrieve() func() {
	// Setup the asset types we want to retrieve data for.
	switch {
	case e.Config.FilterParams.Chassis:
		e.FilterAssetType = append(e.FilterAssetType, "chassis")
	case e.Config.FilterParams.Servers:
		e.FilterAssetType = append(e.FilterAssetType, "servers")
	case !e.Config.FilterParams.Chassis && !e.Config.FilterParams.Servers:
		e.FilterAssetType = []string{"chassis", "servers"}
	}

	// Based on the filter param given, return the asset iterator method.
	switch {
	case e.Config.FilterParams.Serials != "":
		return e.AssetIterBySerial
	case e.Config.FilterParams.Ips != "":
		return e.AssetIterByIP
	default:
		return e.AssetIter
	}
}

// ExecCmd executes the executable with the given args and returns
// if retry is declared, the command is retried for the given number with an interval of 10 seconds,
// the response as a slice of bytes, and the error if any.
func ExecCmd(exe string, args []string, retry int) (out []byte, err error) {
	cmd := exec.Command(exe, args...)

	// To ignore SIGINTs received by bmcbutler, the commands are spawned in their own process group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	out, err = cmd.Output()
	if err != nil && retry == 0 {
		return out, err
	}

	if err != nil && retry > 1 {
		retry--
		time.Sleep(time.Second * 10)
		return ExecCmd(exe, args, retry)
	}

	return out, err
}

// SetChassisInstalled is a method used to update a chassis state in the inventory.
func (e *Enc) SetChassisInstalled(serials string) {
	log := e.Log
	component := "SetChassisInstalled"

	// assetlookup inventory --set-chassis-installed FOO123,BAR123
	cmdArgs := []string{"inventory", "--set-chassis-installed", serials}

	encBin := e.Config.Inventory.Enc.Bin
	out, err := ExecCmd(encBin, cmdArgs, 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
			"cmd":       fmt.Sprintf("%s %s", encBin, strings.Join(cmdArgs, " ")),
			"output":    fmt.Sprintf("%s", out),
		}).Warn("Command to update chassis state returned error.")
	}
}

// nolint: gocyclo
func (e *Enc) encQueryBySerial(serials string) (assets []asset.Asset) {
	log := e.Log
	component := "encQueryBySerial"

	// assetlookup enc --serials FOO123,BAR123
	cmdArgs := []string{"enc", "--serials", serials}

	encBin := e.Config.Inventory.Enc.Bin
	out, err := ExecCmd(encBin, cmdArgs, 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
			"cmd":       fmt.Sprintf("%s %s", encBin, strings.Join(cmdArgs, " ")),
			"output":    fmt.Sprintf("%s", out),
		}).Fatal("Inventory query failed, lookup command returned error.")
	}

	cmdResp := AssetAttributes{}
	err = json.Unmarshal(out, &cmdResp)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
			"cmd":       fmt.Sprintf("%s %s", encBin, strings.Join(cmdArgs, " ")),
			"output":    fmt.Sprintf("%s", out),
		}).Fatal("JSON Unmarshal command response returned error.")
	}

	if len(cmdResp.Data) == 0 {
		log.WithFields(logrus.Fields{
			"component": component,
			"Serial(s)": serials,
		}).Warn("No assets returned by inventory for given serial(s).")

		return []asset.Asset{}
	}

	missingSerials := strings.Split(serials, ",")
	for serial, attributes := range cmdResp.Data {
		attributes := e.SetBMCInterfaces(attributes)
		if len(attributes.BMCIPAddresses) == 0 {
			metrics.IncrCounter([]string{"inventory", "assets_noip_enc"}, 1)
			continue
		}

		// missing Serials are Serials we looked up using the enc and got no data for.
		for idx, s := range missingSerials {
			if s == serial {
				// if its in the list, purge it.
				missingSerials = append(missingSerials[:idx], missingSerials[idx+1:]...)
			}
		}

		extras := AttributesExtrasAsMap(attributes.Extras)
		assets = append(assets,
			asset.Asset{
				IPAddresses: attributes.BMCIPAddresses,
				Serial:      serial,
				Location:    attributes.Location,
				Extra:       extras,
			})
	}

	// append missing Serials to assets
	if len(missingSerials) > 0 {
		for _, serial := range missingSerials {
			assets = append(assets, asset.Asset{Serial: serial, IPAddresses: []string{}})
		}
	}

	metrics.IncrCounter([]string{"inventory", "assets_fetched_enc"}, int64(len(assets)))

	return assets
}

// nolint: gocyclo
func (e *Enc) encQueryByIP(ips string) (assets []asset.Asset) {
	log := e.Log
	component := "encQueryByIP"

	// if no attributes can be received we return assets objs
	// populate and return slice of assets with no attributes except ips.
	populateAssetsWithNoAttributes := func() {
		ipList := strings.Split(ips, ",")
		for _, ip := range ipList {
			assets = append(assets, asset.Asset{IPAddresses: []string{ip}})
		}
	}

	// assetlookup enc --serials 192.168.1.1,192.168.1.2
	cmdArgs := []string{"enc", "--ips", ips}

	encBin := e.Config.Inventory.Enc.Bin
	out, err := ExecCmd(encBin, cmdArgs, 0)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
			"cmd":       fmt.Sprintf("%s %s", encBin, strings.Join(cmdArgs, " ")),
			"output":    fmt.Sprintf("%s", out),
		}).Warn("Inventory query failed, lookup command returned error.")

		populateAssetsWithNoAttributes()
		return assets
	}

	cmdResp := AssetAttributes{}
	err = json.Unmarshal(out, &cmdResp)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
			"cmd":       fmt.Sprintf("%s %s", encBin, strings.Join(cmdArgs, " ")),
			"output":    fmt.Sprintf("%s", out),
		}).Fatal("JSON Unmarshal command response returned error.")
	}

	if len(cmdResp.Data) == 0 {
		log.WithFields(logrus.Fields{
			"component": component,
			"IP(s)":     ips,
		}).Debug("No assets returned by inventory for given IP(s).")

		populateAssetsWithNoAttributes()
		return assets
	}

	// missing IPs are IPs we looked up using the enc and got no data for.
	missingIPs := strings.Split(ips, ",")
	for serial, attributes := range cmdResp.Data {
		attributes := e.SetBMCInterfaces(attributes)
		if len(attributes.BMCIPAddresses) == 0 {
			populateAssetsWithNoAttributes()
			metrics.IncrCounter([]string{"inventory", "assets_noip_enc"}, 1)
			continue
		}

		for _, bmcIPAddress := range attributes.BMCIPAddresses {
			for idx, ip := range missingIPs {
				if ip == bmcIPAddress {
					missingIPs = append(missingIPs[:idx], missingIPs[idx+1:]...)
				}
			}
		}
		extras := AttributesExtrasAsMap(attributes.Extras)

		assets = append(assets,
			asset.Asset{
				IPAddresses: attributes.BMCIPAddresses,
				Serial:      serial,
				Location:    attributes.Location,
				Extra:       extras,
			})
	}

	// append missing IPs.
	if len(missingIPs) > 0 {
		for _, ip := range missingIPs {
			assets = append(assets, asset.Asset{IPAddresses: []string{ip}})
		}
	}

	metrics.IncrCounter([]string{"inventory", "assets_fetched_enc"}, int64(len(assets)))

	return assets
}

// encQueryByOffset returns a slice of assets and if the query reached the end of assets.
// assetType is one of 'servers/chassis'
// location is a comma delimited list of locations
func (e *Enc) encQueryByOffset(assetType string, offset int, limit int, location string) (assets []asset.Asset, endOfAssets bool) {
	component := "EncQueryByOffset"
	log := e.Log

	assets = make([]asset.Asset, 0)

	var encAssetTypeFlag string

	switch assetType {
	case "servers":
		encAssetTypeFlag = "--server"
	case "chassis":
		encAssetTypeFlag = "--chassis"
	case "discretes":
		encAssetTypeFlag = "--server"
	}

	// assetlookup inventory --server --offset 0 --limit 10
	cmdArgs := []string{
		"inventory", encAssetTypeFlag,
		"--limit", strconv.Itoa(limit),
		"--offset", strconv.Itoa(offset),
	}

	//--location ams9
	if location != "" {
		cmdArgs = append(cmdArgs, "--location")
		cmdArgs = append(cmdArgs, location)
	}

	encBin := e.Config.Inventory.Enc.Bin
	out, err := ExecCmd(encBin, cmdArgs, 3)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
			"cmd":       fmt.Sprintf("%s %s", encBin, strings.Join(cmdArgs, " ")),
			"output":    fmt.Sprintf("%s", out),
		}).Fatal("Inventory query failed, lookup command returned error.")
	}

	cmdResp := AssetAttributes{}
	err = json.Unmarshal(out, &cmdResp)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
			"cmd":       fmt.Sprintf("%s %s", encBin, strings.Join(cmdArgs, " ")),
			"output":    fmt.Sprintf("%s", out),
		}).Fatal("JSON Unmarshal command response returned error.")
	}

	endOfAssets = cmdResp.EndOfAssets

	if len(cmdResp.Data) == 0 {
		return []asset.Asset{}, endOfAssets
	}

	for serial, attributes := range cmdResp.Data {
		attributes := e.SetBMCInterfaces(attributes)
		if len(attributes.BMCIPAddresses) == 0 {
			metrics.IncrCounter([]string{"inventory", "assets_noip_enc"}, 1)
			continue
		}

		extras := AttributesExtrasAsMap(attributes.Extras)
		assets = append(assets,
			asset.Asset{
				IPAddresses: attributes.BMCIPAddresses,
				Serial:      serial,
				Type:        assetType,
				Location:    attributes.Location,
				Extra:       extras,
			})
	}

	metrics.IncrCounter([]string{"inventory", "assets_fetched_enc"}, int64(len(assets)))

	return assets, endOfAssets
}

// AssetIter fetches assets and sends them over the asset channel.
// Iter stuffs assets into an array of Assets, writes that to the channel.
func (e *Enc) AssetIter() {
	var interrupt bool

	go func() { <-e.StopChan; interrupt = true }()

	defer close(e.AssetsChan)
	//defer d.MetricsEmitter.MeasureSince(component, time.Now())

	locations := strings.Join(e.Config.Locations, ",")
	for _, assetType := range e.FilterAssetType {
		limit := e.BatchSize
		offset := 0

		for {
			var endOfAssets bool

			assets, endOfAssets := e.encQueryByOffset(assetType, offset, limit, locations)

			e.Log.WithFields(logrus.Fields{
				"component": "inventory",
				"method":    "AssetIter",
				"Asset":     assetType,
				"Offset":    offset,
				"Limit":     limit,
				"locations": locations,
			}).Debug("Assets retrieved.")

			e.AssetsChan <- assets

			// Increment offset for the next set of assets.
			offset += limit

			// ENC indicates we've reached the end of assets?
			if endOfAssets || interrupt {
				e.Log.WithFields(logrus.Fields{
					"component": "inventory",
					"method":    "AssetIter",
				}).Debug("Reached end of assets/interrupt received.")
				break
			}
		}
	}
}

// Reads the list of serials passed by the user via CLI.
// Queries ENC for the serials, then passes them to the assets channel.
func (e *Enc) AssetIterBySerial() {
	defer close(e.AssetsChan)

	serials := e.Config.FilterParams.Serials
	assets := e.encQueryBySerial(serials)
	e.AssetsChan <- assets
}

// Reads the list of IPs passed by the user via CLI.
// Queries ENC for attributes related to those, then passes them to the assets channel.
// If no attributes for a given IP are returned, an asset with just the IP is returned.
func (e *Enc) AssetIterByIP() {
	defer close(e.AssetsChan)

	ips := e.Config.FilterParams.Ips
	assets := e.encQueryByIP(ips)
	e.AssetsChan <- assets
}
