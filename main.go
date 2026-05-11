// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"

	harvesterBuilder "github.com/hulto/packer-plugin-harvester/builder/harvester"
	"github.com/hulto/packer-plugin-harvester/builder/scaffolding"
	scaffoldingData "github.com/hulto/packer-plugin-harvester/datasource/scaffolding"
	scaffoldingPP "github.com/hulto/packer-plugin-harvester/post-processor/scaffolding"
	scaffoldingProv "github.com/hulto/packer-plugin-harvester/provisioner/scaffolding"
	scaffoldingVersion "github.com/hulto/packer-plugin-harvester/version"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("my-builder", new(scaffolding.Builder))
	pps.RegisterBuilder("iso", new(harvesterBuilder.ISOBuilder))
	pps.RegisterBuilder("clone", new(harvesterBuilder.CloneBuilder))
	pps.RegisterProvisioner("my-provisioner", new(scaffoldingProv.Provisioner))
	pps.RegisterPostProcessor("my-post-processor", new(scaffoldingPP.PostProcessor))
	pps.RegisterDatasource("my-datasource", new(scaffoldingData.Datasource))
	pps.SetVersion(scaffoldingVersion.PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
