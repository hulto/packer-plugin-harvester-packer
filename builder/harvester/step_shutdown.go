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

// StepShutdown shuts down the build VM gracefully before creating the output
// image. If a ShutdownCommand is configured it is run inside the guest via the
// communicator; otherwise a graceful stop is requested via the Harvester API.
type StepShutdown struct {
	Config *Config
}

// Run executes the shutdown step.
func (s *StepShutdown) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	vmName, ok := state.Get("vm_name").(string)
	if !ok || vmName == "" {
		ui.Error("vm_name not found in state")
		state.Put("error", fmt.Errorf("vm_name missing from state"))
		return multistep.ActionHalt
	}

	// If a custom shutdown command is configured, run it via the communicator.
	if s.Config.ShutdownCommand != "" {
		comm, ok := state.GetOk("communicator")
		if ok && comm != nil {
			ui.Say(fmt.Sprintf("Running shutdown command: %s", s.Config.ShutdownCommand))
			cmd := &packersdk.RemoteCmd{Command: s.Config.ShutdownCommand}
			if err := cmd.RunWithUi(ctx, comm.(packersdk.Communicator), ui); err != nil {
				ui.Error(fmt.Sprintf("Failed to run shutdown command: %s", err))
				// Fall through to API-based shutdown.
			}
		}
	}

	ui.Say(fmt.Sprintf("Requesting graceful stop for VM %q via API...", vmName))
	if err := client.StopVM(vmName); err != nil {
		ui.Error(fmt.Sprintf("Failed to stop VM %q: %s", vmName, err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Wait for the VM to stop.
	timeout := s.Config.ShutdownTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ui.Say(fmt.Sprintf("Waiting up to %s for VM %q to stop...", timeout, vmName))

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			state.Put("error", ctx.Err())
			return multistep.ActionHalt
		default:
		}

		vm, err := client.GetVM(vmName)
		if err != nil {
			ui.Say(fmt.Sprintf("Error querying VM status: %s, retrying...", err))
			time.Sleep(5 * time.Second)
			continue
		}

		// VirtualMachine prints "Stopped" when the VMI is gone.
		phase := vm.Status.Phase
		ui.Say(fmt.Sprintf("VM printable status: %s", phase))
		if phase == "Stopped" || phase == "" {
			// Double-check by trying to get the VMI – if it's gone, we're done.
			if _, err := client.GetVMI(vmName); err != nil {
				ui.Say(fmt.Sprintf("VM %q has stopped.", vmName))
				return multistep.ActionContinue
			}
		}

		time.Sleep(5 * time.Second)
	}

	err := fmt.Errorf("timeout waiting for VM %q to stop after %s", vmName, timeout)
	state.Put("error", err)
	return multistep.ActionHalt
}

// Cleanup is a no-op.
func (s *StepShutdown) Cleanup(_ multistep.StateBag) {}
