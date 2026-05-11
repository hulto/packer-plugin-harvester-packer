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

// StepWaitForInstance waits until the VirtualMachineInstance (VMI) transitions
// to the Running phase and has a routable IP address.
type StepWaitForInstance struct {
	Config *Config
}

// Run polls the VMI until it is Running or the timeout is exceeded.
func (s *StepWaitForInstance) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	vmName, ok := state.Get("vm_name").(string)
	if !ok || vmName == "" {
		ui.Error("vm_name not found in state; StepCreateVM must run before StepWaitForInstance")
		state.Put("error", fmt.Errorf("vm_name missing from state"))
		return multistep.ActionHalt
	}

	timeout := s.Config.WaitForInstanceTimeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ui.Say(fmt.Sprintf("Waiting up to %s for VM %q to reach Running state...", timeout, vmName))

	deadline := time.Now().Add(timeout)
	tickInterval := 5 * time.Second

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			state.Put("error", ctx.Err())
			return multistep.ActionHalt
		default:
		}

		vmi, err := client.GetVMI(vmName)
		if err != nil {
			ui.Say(fmt.Sprintf("VMI not yet available (%s), retrying...", err))
			time.Sleep(tickInterval)
			continue
		}

		phase := string(vmi.Status.Phase)
		ui.Say(fmt.Sprintf("VMI phase: %s", phase))

		if vmi.Status.Phase == hvclient.VMIPhaseFailed {
			err := fmt.Errorf("VMI %q entered Failed phase", vmName)
			state.Put("error", err)
			return multistep.ActionHalt
		}

		if vmi.Status.Phase == hvclient.VMIPhaseRunning {
			// Try to find an IP address.
			ip := extractIP(vmi)
			if ip != "" {
				ui.Say(fmt.Sprintf("VM %q is running with IP %s", vmName, ip))
				state.Put("vm_ip", ip)
				return multistep.ActionContinue
			}
			ui.Say("VMI is Running but no IP yet, waiting...")
		}

		time.Sleep(tickInterval)
	}

	err := fmt.Errorf("timeout waiting for VMI %q to reach Running state after %s", vmName, timeout)
	state.Put("error", err)
	return multistep.ActionHalt
}

// Cleanup is a no-op; VM cleanup is handled by StepCreateVM.
func (s *StepWaitForInstance) Cleanup(_ multistep.StateBag) {}

// extractIP returns the first IP address found on any VMI interface.
func extractIP(vmi *hvclient.VirtualMachineInstance) string {
	for _, iface := range vmi.Status.Interfaces {
		if iface.IPAddress != "" {
			return iface.IPAddress
		}
	}
	return ""
}
