// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	hvclient "github.com/hashicorp/packer-plugin-scaffolding/builder/harvester/client"
)

// StepCreateCDImage builds an auxiliary ISO from cd_files/cd_content and
// imports it into Harvester as a temporary VirtualMachineImage.
type StepCreateCDImage struct {
	Config *Config

	cdStep              *commonsteps.StepCreateCD
	cdImageName         string
	createdCDImageNames []string
}

// Run creates and uploads the temporary CD image used by the ISO builder.
func (s *StepCreateCDImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if !s.Config.hasCDConfig() {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	ui.Say("Preparing auxiliary CD image from cd_files/cd_content...")

	s.cdStep = &commonsteps.StepCreateCD{
		Files:   s.Config.CDFiles,
		Content: s.Config.CDContent,
		Label:   s.Config.CDLabel,
	}
	if action := s.cdStep.Run(ctx, state); action != multistep.ActionContinue {
		return action
	}

	cdPathRaw, ok := state.GetOk("cd_path")
	if !ok {
		err := fmt.Errorf("cd_path missing after CD creation")
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	cdPath, ok := cdPathRaw.(string)
	if !ok || cdPath == "" {
		err := fmt.Errorf("invalid cd_path after CD creation")
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	if err := s.createAndUploadCDImage(ctx, ui, client, cdPath, s.Config.Namespace); err != nil {
		ui.Error(fmt.Sprintf("Failed to import auxiliary CD image: %s", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	state.Put("cd_image_name", s.cdImageName)
	state.Put("cd_image_namespace", s.Config.Namespace)
	ui.Say(fmt.Sprintf("Auxiliary CD image %q is ready in namespace %q", s.cdImageName, s.Config.Namespace))

	return multistep.ActionContinue
}

func (s *StepCreateCDImage) createAndUploadCDImage(
	ctx context.Context,
	ui packersdk.Ui,
	client *hvclient.HarvesterClient,
	cdPath, namespace string,
) error {
	file, err := os.Open(cdPath)
	if err != nil {
		return fmt.Errorf("open generated CD ISO %q: %w", cdPath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat generated CD ISO %q: %w", cdPath, err)
	}
	if stat.Size() <= 0 {
		return fmt.Errorf("generated CD ISO %q is empty", cdPath)
	}

	s.cdImageName = fmt.Sprintf("packer-cd-%s", randomHex(8))
	s.createdCDImageNames = append(s.createdCDImageNames, s.cdImageName)

	ui.Say(fmt.Sprintf("Creating auxiliary CD image %q in Harvester...", s.cdImageName))

	img := &hvclient.VirtualMachineImage{
		TypeMeta: hvclient.TypeMeta{
			APIVersion: "harvesterhci.io/v1beta1",
			Kind:       "VirtualMachineImage",
		},
		ObjectMeta: hvclient.ObjectMeta{
			Name:      s.cdImageName,
			Namespace: namespace,
		},
		Spec: hvclient.VirtualMachineImageSpec{
			DisplayName: s.cdImageName,
			SourceType:  "upload",
		},
	}

	if _, err := client.CreateVMImage(img); err != nil {
		return fmt.Errorf("create VirtualMachineImage: %w", err)
	}

	ui.Say(fmt.Sprintf("Waiting for image %q to be ready to accept upload...", s.cdImageName))
	if err := waitForVMImageInitialized(ctx, client, namespace, s.cdImageName, ui, 3*time.Minute); err != nil {
		return fmt.Errorf("image %q did not initialize: %w", s.cdImageName, err)
	}

	ui.Say(fmt.Sprintf("Uploading generated CD ISO to Harvester image %q...", s.cdImageName))
	if err := client.UploadVMImage(namespace, s.cdImageName, file, stat.Size()); err != nil {
		return fmt.Errorf("upload CD ISO to VirtualMachineImage %q: %w", s.cdImageName, err)
	}

	if err := waitForVMImageActive(ctx, client, namespace, s.cdImageName, ui, 20*time.Minute); err != nil {
		return err
	}

	return nil
}

// Cleanup removes temporary local artifacts and the imported CD image.
func (s *StepCreateCDImage) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	if s.cdStep != nil {
		s.cdStep.Cleanup(state)
	}

	seen := map[string]bool{}
	for _, name := range s.createdCDImageNames {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		ui.Say(fmt.Sprintf("Removing temporary auxiliary CD image %q...", name))
		if err := deleteVMImageWithRetry(ui, client, s.Config.Namespace, name, 2*time.Minute); err != nil {
			ui.Error(fmt.Sprintf("Warning: failed to delete temporary CD image %q: %s", name, err))
		}
	}

}

func deleteVMImageWithRetry(
	ui packersdk.Ui,
	client *hvclient.HarvesterClient,
	namespace, name string,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)
	loggedInUseWait := false

	for {
		err := client.DeleteVMImage(namespace, name)
		switch {
		case err == nil:
			return nil
		case hvclient.IsNotFoundError(err):
			return nil
		case isVMImageInUseError(err):
			if time.Now().After(deadline) {
				return err
			}
			if !loggedInUseWait {
				ui.Say(fmt.Sprintf("Temporary CD image %q is still referenced by a volume; waiting for release before retrying deletion...", name))
				loggedInUseWait = true
			}
			time.Sleep(5 * time.Second)
		default:
			return err
		}
	}
}

func isVMImageInUseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "being used by volume")
}

func waitForVMImageInitialized(
	ctx context.Context,
	client *hvclient.HarvesterClient,
	namespace, name string,
	ui packersdk.Ui,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		img, err := client.GetVMImage(namespace, name)
		if err != nil {
			ui.Say(fmt.Sprintf("Waiting for image %q to initialize: %s", name, err))
			time.Sleep(5 * time.Second)
			continue
		}
		if img.Status.Phase == "Failed" {
			return fmt.Errorf("image %q failed during initialization: %s", name, img.Status.Message)
		}
		for _, cond := range img.Status.Conditions {
			if cond.Type == "Initialized" && cond.Status == "True" {
				return nil
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timeout waiting for image %q to reach Initialized=True", name)
}

func waitForVMImageActive(
	ctx context.Context,
	client *hvclient.HarvesterClient,
	namespace, name string,
	ui packersdk.Ui,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)
	lastPhase := ""
	lastProgress := -1
	lastAdvance := time.Now()

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
		progress := img.Status.Progress
		if phase != lastPhase {
			ui.Say(fmt.Sprintf("Image %q phase: %s", name, phase))
			lastPhase = phase
		}
		if progress != lastProgress {
			ui.Say(fmt.Sprintf("Image %q progress: %d%%", name, progress))
			lastProgress = progress
			lastAdvance = time.Now()
		}

		if phase == "Active" || phase == "Completed" || phase == "Ready" {
			return nil
		}
		// Harvester backing-image uploads set Imported=True rather than a phase field.
		for _, cond := range img.Status.Conditions {
			if cond.Type == "Imported" && cond.Status == "True" {
				return nil
			}
		}
		if (phase == "Uploading" || phase == "Importing" || phase == "Pending") && time.Since(lastAdvance) > 2*time.Minute {
			return fmt.Errorf("image %q stalled in phase %q at %d%%", name, phase, progress)
		}
		if phase == "Failed" {
			return fmt.Errorf("image %q entered Failed phase: %s", name, img.Status.Message)
		}

		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("timeout waiting for image %q to become Active", name)
}
