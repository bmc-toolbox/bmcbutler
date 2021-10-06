package configure

import (
	"strings"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmclib/cfgresources"
	"github.com/bmc-toolbox/bmclib/devices"
	"github.com/sirupsen/logrus"
)

// Cmc struct declares attributes required to apply configuration.
type Cmc struct {
	asset        *asset.Asset
	bmc          devices.Cmc
	resources    []string
	configure    devices.Configure
	config       *cfgresources.ResourcesConfig
	logger       *logrus.Logger
	ip           string
	serial       string
	vendor       string
	hardwareType string
	stopChan     <-chan struct{}
}

// NewCmcConfigurator returns a new configure struct to apply configuration.
func NewCmcConfigurator(bmc devices.Cmc,
	asset *asset.Asset,
	resources []string,
	config *cfgresources.ResourcesConfig,
	stopChan <-chan struct{},
	logger *logrus.Logger) *Cmc {

	return &Cmc{
		// asset to be setup
		asset: asset,
		// client is of type devices.Bmc
		bmc: bmc,
		// if --resources was passed, only these resources will be applied
		resources: resources,
		// devices.Cmc is type asserted to apply configuration,
		// this is possible since devices.Bmc embeds the Configure interface.
		configure:    bmc.(devices.Configure),
		config:       config,
		logger:       logger,
		stopChan:     stopChan,
		ip:           asset.IPAddress,
		serial:       asset.Serial,
		vendor:       asset.Vendor,
		hardwareType: asset.HardwareType,
	}
}

// Apply applies configuration.
func (b *Cmc) Apply() { //nolint: gocyclo

	var interrupt bool
	go func() { <-b.stopChan; interrupt = true }()

	// slice of configuration resources to be applied.
	var resources []string

	// retrieve valid or known configuration resources for the bmc.
	if len(b.resources) > 0 {
		resources = b.resources
	} else {
		resources = b.configure.Resources()
	}

	b.ip = b.asset.IPAddress

	var failed, success []string

	b.logger.WithFields(logrus.Fields{
		"Vendor":       b.vendor,
		"HardwareType": b.hardwareType,
		"Serial":       b.serial,
		"IPAddress":    b.ip,
		"To apply":     strings.Join(resources, ", "),
	}).Trace("Configuration resources to be applied.")

	for _, resource := range resources {

		var err error

		// check if an interrupt was received.
		if interrupt == true {
			b.logger.WithFields(logrus.Fields{
				"Vendor":       b.vendor,
				"HardwareType": b.hardwareType,
				"Serial":       b.serial,
				"IPAddress":    b.ip,
			}).Debug("Received interrupt.")
			break
		}

		switch resource {
		case "user":
			if b.config.User != nil {
				err = b.configure.User(b.config.User)
			}
		case "syslog":
			if b.config.Syslog != nil {
				err = b.configure.Syslog(b.config.Syslog)
			}
		case "ntp":
			if b.config.Ntp != nil {
				err = b.configure.Ntp(b.config.Ntp)
			}
		case "ldap":
			if b.config.Ldap != nil {
				err = b.configure.Ldap(b.config.Ldap)
			}
		case "ldap_group":
			if b.config.LdapGroups != nil && b.config.Ldap != nil {
				o, err := b.config.LdapGroups.GetExtraGroups(b.asset.Serial, b.asset.Vendor)
				if err != nil {
					b.logger.WithFields(logrus.Fields{
						"Vendor":       b.vendor,
						"HardwareType": b.hardwareType,
						"Serial":       b.serial,
						"IPAddress":    b.ip,
						"Error":        err,
						"Output":       o,
						"Groups":       b.config.LdapGroups.Groups,
					}).Warn("Trying to fetch more LDAP groups has failed.")
				}
				err = b.configure.LdapGroups(b.config.LdapGroups.Groups, b.config.Ldap)
			}
		case "license":
			if b.config.License != nil {
				err = b.configure.SetLicense(b.config.License)
			}
		case "network":
			if b.config.Network != nil {
				_, err = b.configure.Network(b.config.Network)
			}
		default:
			b.logger.WithFields(logrus.Fields{
				"resource": resource,
			}).Warn("Unknown resource.")
		}

		if err != nil {
			failed = append(failed, resource)
			b.logger.WithFields(logrus.Fields{
				"resource":     resource,
				"Vendor":       b.vendor,
				"HardwareType": b.hardwareType,
				"Serial":       b.serial,
				"IPAddress":    b.ip,
				"Error":        err,
			}).Warn("Resource configuration returned errors.")
		} else {
			success = append(success, resource)
		}

		b.logger.WithFields(logrus.Fields{
			"resource":     resource,
			"Vendor":       b.vendor,
			"HardwareType": b.hardwareType,
			"Serial":       b.serial,
			"IPAddress":    b.ip,
		}).Trace("Resource configuration applied.")

	}

	if len(failed) > 0 {
		b.logger.WithFields(logrus.Fields{
			"Vendor":       b.vendor,
			"HardwareType": b.hardwareType,
			"Serial":       b.serial,
			"IPAddress":    b.ip,
			"success":      false,
			"applied":      strings.Join(success, ", "),
			"failed":       strings.Join(failed, ", "),
		}).Warn("One or more resources failed to apply.")
		return
	}

	b.logger.WithFields(logrus.Fields{
		"Vendor":       b.vendor,
		"HardwareType": b.hardwareType,
		"Serial":       b.serial,
		"IPAddress":    b.ip,
		"success":      true,
		"applied":      strings.Join(success, ", "),
	}).Info("CMC configuration actions successful.")
}
