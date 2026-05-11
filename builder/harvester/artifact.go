// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package harvester

import "fmt"

const BuilderID = "harvester.builder"

// Artifact is the packer artifact produced by the Harvester builder. It
// represents a VirtualMachineImage in Harvester.
type Artifact struct {
	// ImageName is the metadata name of the VirtualMachineImage.
	ImageName string
	// ImageNamespace is the Kubernetes namespace of the VirtualMachineImage.
	ImageNamespace string
	// HarvesterURL is the base URL of the Harvester server where the image lives.
	HarvesterURL string
	// StateData holds data shared with post-processors.
	StateData map[string]interface{}
}

func (*Artifact) BuilderId() string {
	return BuilderID
}

func (a *Artifact) Files() []string {
	return []string{}
}

func (a *Artifact) Id() string {
	return fmt.Sprintf("%s/%s", a.ImageNamespace, a.ImageName)
}

func (a *Artifact) String() string {
	return fmt.Sprintf("Harvester image %s/%s on %s", a.ImageNamespace, a.ImageName, a.HarvesterURL)
}

func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

// Destroy does not delete the Harvester image by default. Packer treats
// "destroy" as deleting local artefacts; the remote image is kept.
func (a *Artifact) Destroy() error {
	return nil
}
