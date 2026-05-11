// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"

	harvesterBuilder "github.com/hulto/packer-plugin-harvester/builder/harvester"
	"github.com/hulto/packer-plugin-harvester/version"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("iso", new(harvesterBuilder.ISOBuilder))
	pps.RegisterBuilder("clone", new(harvesterBuilder.CloneBuilder))
	pps.SetVersion(version.PluginVersion)
	err := pps.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
