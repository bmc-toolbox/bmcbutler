// Copyright © 2018 Joel Rebello <joel.rebello@booking.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/gobuffalo/plush"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/bmc-toolbox/bmcbutler/pkg/asset"
	"github.com/bmc-toolbox/bmcbutler/pkg/secrets"

	"github.com/bmc-toolbox/bmclib/cfgresources"
)

// Resource struct holds configuration resource related attributes
// Config resources are configuration parameters butlers apply to assets.
type Resource struct {
	Log     *logrus.Logger
	Asset   *asset.Asset
	Secrets *secrets.Store
}

// ReadYamlTemplate reads the given config .yml file, returns it as a slice of bytes.
func ReadYamlTemplate(yamlFile string) (yamlTemplate []byte, err error) {
	// Does the file exist?
	_, err = os.Stat(yamlFile)
	if err != nil {
		return []byte{}, err
	}

	// Read the file.
	yamlTemplate, err = ioutil.ReadFile(yamlFile)
	if err != nil {
		return []byte{}, err
	}

	return yamlTemplate, nil
}

// RenderYamlTemplate renders templated values in the given config .yml, returns it as a slice of bytes.
func (r *Resource) RenderYamlTemplate(yamlTemplate []byte) (yamlData []byte) {
	log := r.Log
	component := "RenderYamlTemplate"

	// Rendering templated data.
	ctx := plush.NewContext()

	// Assign variables exposed in the template.
	ctx.Set("vendor", strings.ToLower(r.Asset.Vendor))
	ctx.Set("location", strings.ToLower(r.Asset.Location))
	ctx.Set("assetType", strings.ToLower(r.Asset.Type))
	ctx.Set("hardwareType", strings.ToLower(r.Asset.HardwareType))
	ctx.Set("serial", strings.ToLower(r.Asset.Serial))
	ctx.Set("ipaddress", strings.ToLower(r.Asset.IPAddress))
	ctx.Set("extra", r.Asset.Extra)

	// r.Secrets is non nil if the bmcbutler.yml declares secretsFromVault: true.
	if r.Secrets != nil {
		ctx.Set("lookup_secret", func(s string) string {
			secret, _ := r.Secrets.Get(s)
			return secret
		})
	}

	// Render, plush is awesome!
	s, err := plush.Render(string(yamlTemplate), ctx)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
		}).Fatal("Error rendering configuration yml template.")
	}

	return []byte(s)
}

// LoadConfigResources gets the template rendered and unmarshals the resulting yml.
func (r *Resource) LoadConfigResources(yamlTemplate []byte) (config *cfgresources.ResourcesConfig) {
	component := "LoadConfigResources"
	log := r.Log

	yamlData := r.RenderYamlTemplate(yamlTemplate)

	err := yaml.Unmarshal(yamlData, &config)
	if err != nil {
		log.WithFields(logrus.Fields{
			"component": component,
			"Error":     err,
		}).Fatal("Unable to Unmarshal config resources template.")
	}

	return config
}
