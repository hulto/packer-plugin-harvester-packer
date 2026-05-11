// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	hvclient "github.com/hulto/packer-plugin-harvester/builder/harvester/client"
)

// StepCreateImage exports the build VM's root disk as a new
// VirtualMachineImage in Harvester.
type StepCreateImage struct {
	Config    *Config
	imageName string
}

// Run creates the output VirtualMachineImage.
func (s *StepCreateImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	vmName, ok := state.Get("vm_name").(string)
	if !ok || vmName == "" {
		ui.Error("vm_name not found in state")
		state.Put("error", fmt.Errorf("vm_name missing from state"))
		return multistep.ActionHalt
	}

	cfg := s.Config
	displayName := cfg.OutputImageName
	if displayName == "" {
		displayName = "packer-" + vmName
	}

	// The root disk PVC has the same name as <vmName>-disk-0 (set in StepCreateVM).
	rootDiskPVC := vmName + "-disk-0"

	s.imageName = fmt.Sprintf("packer-%s", randomHex(8))

	ui.Say(fmt.Sprintf("Creating Harvester image %q from PVC %q...", displayName, rootDiskPVC))

	img := &hvclient.VirtualMachineImage{
		TypeMeta: hvclient.TypeMeta{
			APIVersion: "harvesterhci.io/v1beta1",
			Kind:       "VirtualMachineImage",
		},
		ObjectMeta: hvclient.ObjectMeta{
			Name:      s.imageName,
			Namespace: cfg.OutputImageNamespace,
		},
		Spec: hvclient.VirtualMachineImageSpec{
			DisplayName:  displayName,
			SourceType:   "export-from-volume",
			PVCName:      rootDiskPVC,
			PVCNamespace: cfg.Namespace,
		},
	}

	created, err := client.CreateVMImage(img)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to create VirtualMachineImage: %s", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("VirtualMachineImage %q created, waiting for it to be ready...", created.ObjectMeta.Name))

	// Wait for image to become Active.
	if err := s.waitForImage(ctx, client, cfg.OutputImageNamespace, s.imageName, ui); err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Image %q is ready in namespace %q", displayName, cfg.OutputImageNamespace))
	state.Put("image_name", s.imageName)
	state.Put("image_display_name", displayName)
	return multistep.ActionContinue
}

// waitForImage polls until the VirtualMachineImage phase is Active.
func (s *StepCreateImage) waitForImage(ctx context.Context, client *hvclient.HarvesterClient,
	namespace, name string, ui packersdk.Ui) error {

	timeout := 30 * time.Minute
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		img, err := client.GetVMImage(namespace, name)
		if err != nil {
			ui.Say(fmt.Sprintf("Waiting for image %q: %s", name, err))
			time.Sleep(10 * time.Second)
			continue
		}

		phase := img.Status.Phase
		ui.Say(fmt.Sprintf("Image phase: %s", phase))
		if phase == "Active" {
			return nil
		}
		if phase == "Failed" {
			return fmt.Errorf("image %q entered Failed phase: %s", name, img.Status.Message)
		}

		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("timeout waiting for image %q to become Active", name)
}

// Cleanup removes the output image if the build failed (but not on success).
func (s *StepCreateImage) Cleanup(state multistep.StateBag) {
	if s.imageName == "" {
		return
	}
	// Only clean up if there was an error.
	if _, ok := state.GetOk("error"); !ok {
		return
	}

	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	ui.Say(fmt.Sprintf("Removing failed image %q...", s.imageName))
	if err := client.DeleteVMImage(s.Config.OutputImageNamespace, s.imageName); err != nil {
		ui.Error(fmt.Sprintf("Warning: failed to delete image %q: %s", s.imageName, err))
	}
}
