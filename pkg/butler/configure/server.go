package configure

import (
	"context"
	"strings"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/config"
	"github.com/bmc-toolbox/bmclib/cfgresources"
	"github.com/bmc-toolbox/bmclib/devices"
	"github.com/sirupsen/logrus"
)

// Bmc struct declares attributes required to apply configuration.
type Bmc struct {
	bmc          devices.Bmc
	asset        *asset.Asset
	resources    []string
	configure    devices.Configure
	config       *cfgresources.ResourcesConfig
	butlerConfig *config.Params
	logger       *logrus.Logger
	ip           string
	serial       string
	vendor       string
	model        string
	stopChan     <-chan struct{}
}

// NewBmcConfigurator returns a new configure struct to apply configuration.
func NewBmcConfigurator(bmc devices.Bmc,
	asset *asset.Asset,
	resources []string,
	config *cfgresources.ResourcesConfig,
	butlerConfig *config.Params,
	stopChan <-chan struct{},
	logger *logrus.Logger) *Bmc {

	return &Bmc{
		// asset to be setup
		asset: asset,
		// client is of type devices.Bmc
		bmc: bmc,
		// devices.Bmc is type asserted to apply configuration,
		// this is possible since devices.Bmc embeds the Configure interface.
		configure: bmc.(devices.Configure),
		// if --resources was passed, only these resources will be applied
		resources:    resources,
		config:       config,
		butlerConfig: butlerConfig,
		logger:       logger,
		stopChan:     stopChan,
		ip:           asset.IPAddress,
		serial:       asset.Serial,
		vendor:       asset.Vendor,
		model:        asset.Model,
	}
}

// Apply applies configuration.
// nolint: gocyclo
func (b *Bmc) Apply() {

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

	// reset causes are appended here
	var resetCause []string

	b.logger.WithFields(logrus.Fields{
		"Vendor":    b.vendor,
		"Model":     b.model,
		"Serial":    b.serial,
		"IPAddress": b.ip,
		"To apply":  strings.Join(resources, ", "),
	}).Trace("Configuration resources to be applied.")

	for _, resource := range resources {

		var err error
		var reset bool

		// check if an interrupt was received.
		if interrupt == true {
			b.logger.WithFields(logrus.Fields{
				"Vendor":    b.vendor,
				"Model":     b.model,
				"Serial":    b.serial,
				"IPAddress": b.ip,
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
			if b.config.LdapGroup != nil && b.config.Ldap != nil {
				err = b.configure.LdapGroup(b.config.LdapGroup, b.config.Ldap)
			}
		case "license":
			if b.config.License != nil {
				err = b.configure.SetLicense(b.config.License)
			}
		case "network":
			if b.config.Network != nil {
				reset, err = b.configure.Network(b.config.Network)
			}
		case "bios":
			if b.config.Bios != nil {
				err = b.configure.Bios(b.config.Bios)
			}
		case "https_cert":
			if b.config.HTTPSCert != nil {
				reset, err = b.certificateSetup()
			}
		case "power":
			if b.config.Power != nil {
				err = b.configure.Power(b.config.Power)
			}
		default:
			b.logger.WithFields(logrus.Fields{
				"resource": resource,
			}).Warn("Unknown resource.")
		}

		if err != nil {
			failed = append(failed, resource)
			b.logger.WithFields(logrus.Fields{
				"resource":  resource,
				"Vendor":    b.vendor,
				"Model":     b.model,
				"Serial":    b.serial,
				"IPAddress": b.ip,
				"Error":     err,
			}).Warn("Resource configuration returned errors.")
		} else {
			success = append(success, resource)
		}

		if reset {
			resetCause = append(resetCause, resource)
		}

		b.logger.WithFields(logrus.Fields{
			"resource":  resource,
			"Vendor":    b.vendor,
			"Model":     b.model,
			"Serial":    b.serial,
			"IPAddress": b.ip,
		}).Trace("Resource configuration applied.")

	}

	//// Reset BMC if needed.
	if len(resetCause) > 0 {

		b.logger.WithFields(logrus.Fields{
			"Vendor":    b.vendor,
			"Model":     b.model,
			"Serial":    b.serial,
			"IPAddress": b.ip,
			"cause":     strings.Join(resetCause, ", "),
		}).Info("BMC to be reset.")

		// Close the current connection - so we don't leave connections hanging.
		b.bmc.Close(context.TODO())

		//// reset BMC using SSH.
		_, err := b.bmc.PowerCycleBmc()
		if err != nil {
			b.logger.WithFields(logrus.Fields{
				"Vendor":    b.vendor,
				"Model":     b.model,
				"Serial":    b.serial,
				"IPAddress": b.ip,
				"Error":     err,
			}).Warn("BMC reset failed.")

		}
	}

	if len(failed) > 0 {
		b.logger.WithFields(logrus.Fields{
			"Vendor":    b.vendor,
			"Model":     b.model,
			"Serial":    b.serial,
			"IPAddress": b.ip,
			"success":   false,
			"applied":   strings.Join(success, ", "),
			"failed":    strings.Join(failed, ", "),
		}).Warn("One or more resources failed to apply.")
		return
	}

	b.logger.WithFields(logrus.Fields{
		"Vendor":    b.vendor,
		"Model":     b.model,
		"Serial":    b.serial,
		"IPAddress": b.ip,
		"success":   true,
		"applied":   strings.Join(success, ", "),
	}).Info("BMC configuration actions successful.")

}
