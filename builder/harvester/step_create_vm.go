// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	hvclient "github.com/hashicorp/packer-plugin-scaffolding/builder/harvester/client"
)

// StepCreateVM creates the temporary build VM in Harvester.
// For the ISO builder, the VM is created with a blank root disk and the ISO
// image as a CDROM.
// For the clone builder, the VM is created with a root disk cloned from the
// source image.
type StepCreateVM struct {
	Config *Config
	vmName string
}

// Run creates the Harvester VirtualMachine.
func (s *StepCreateVM) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	s.vmName = s.Config.VMName
	if s.vmName == "" {
		s.vmName = fmt.Sprintf("packer-%d", time.Now().UnixNano()/int64(time.Millisecond))
	}

	ui.Say(fmt.Sprintf("Creating build VM %q in namespace %q...", s.vmName, s.Config.Namespace))

	vm, err := s.buildVMSpec(client, ui)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to build VM spec: %s", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	created, err := client.CreateVM(vm)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to create VM: %s", err))
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("VM %q created (UID %s)", created.ObjectMeta.Name, created.ObjectMeta.UID))
	state.Put("vm_name", s.vmName)
	ui.Say("VM created with runStrategy=Once; waiting for it to start...")

	return multistep.ActionContinue
}

// Cleanup removes the VM.
// This always runs at build end regardless of success, failure or cancellation.
func (s *StepCreateVM) Cleanup(state multistep.StateBag) {
	if s.vmName == "" {
		return
	}

	ui := state.Get("ui").(packersdk.Ui)
	client := state.Get("client").(*hvclient.HarvesterClient)

	ui.Say(fmt.Sprintf("Cleaning up VM %q...", s.vmName))
	if err := client.DeleteVM(s.vmName); err != nil {
		ui.Error(fmt.Sprintf("Warning: failed to delete VM %q: %s", s.vmName, err))
	}
}

// buildVMSpec constructs the VirtualMachine resource based on builder type.
func (s *StepCreateVM) buildVMSpec(client *hvclient.HarvesterClient, ui packersdk.Ui) (*hvclient.VirtualMachine, error) {
	cfg := s.Config
	memory := fmt.Sprintf("%dMi", cfg.MemoryMB)
	cpu := cfg.CPUCores
	if cpu == 0 {
		cpu = 2
	}

	diskName := s.vmName + "-disk-0"

	// Build network interface spec.
	networks, ifaces := buildNetworkSpec(cfg)

	// Build volume claim templates annotation (Harvester-specific).
	var volClaimTemplates []map[string]interface{}

	// Build disks and volumes.
	var disks []hvclient.DiskTarget
	var volumes []hvclient.Volume
	builderType, err := cfg.effectiveBuilderType()
	if err != nil {
		return nil, err
	}

	switch builderType {
	case BuilderTypeISO:
		// Resolve ISO image to get its PVC/DataVolume reference.
		isoImage, err := client.GetVMImageByDisplayName(cfg.ISOImageNamespace, cfg.ISOImageName)
		if err != nil {
			return nil, fmt.Errorf("iso image %q: %w", cfg.ISOImageName, err)
		}
		isoImageID := fmt.Sprintf("%s/%s", isoImage.ObjectMeta.Namespace, isoImage.ObjectMeta.Name)
		ui.Say(fmt.Sprintf("Using ISO image %q (id: %s)", cfg.ISOImageName, isoImageID))
		isoDiskSize := imageSizeToGi(isoImage.Status.Size)
		rootStorageClass := chooseStorageClass(cfg.StorageClass, isoImage.Status.StorageClassName)
		if rootStorageClass != cfg.StorageClass {
			ui.Say(fmt.Sprintf("Using image storage class %q for root disk", rootStorageClass))
		}
		cdromStorageClass := chooseISOCDROMStorageClass(rootStorageClass, isoImage.Status.StorageClassName)
		if cdromStorageClass != rootStorageClass {
			ui.Say(fmt.Sprintf("Using image storage class %q for ISO CDROM", cdromStorageClass))
		}

		// Root disk – blank DataVolume created via volume claim template.
		volClaimTemplates = append(volClaimTemplates, buildVolumeClaimTemplate(
			diskName, cfg.DiskSize, rootStorageClass, "", cfg.Namespace,
		))
		disks = append(disks, hvclient.DiskTarget{
			Name:      "disk-0",
			BootOrder: 1,
			Disk:      &hvclient.Disk{Bus: "virtio"},
		})
		volumes = append(volumes, hvclient.Volume{
			Name:                  "disk-0",
			PersistentVolumeClaim: &hvclient.PersistentVolumeClaimVolumeSource{ClaimName: diskName},
		})

		// ISO CDROM.
		cdromPVCName := fmt.Sprintf("packer-iso-%s", randomHex(6))
		volClaimTemplates = append(volClaimTemplates, buildVolumeClaimTemplate(
			cdromPVCName, isoDiskSize, cdromStorageClass, isoImageID, cfg.Namespace,
		))
		disks = append(disks, hvclient.DiskTarget{
			Name:      "cdrom-0",
			BootOrder: 2,
			CDRom:     &hvclient.CDRom{Bus: "sata"},
		})
		volumes = append(volumes, hvclient.Volume{
			Name:                  "cdrom-0",
			PersistentVolumeClaim: &hvclient.PersistentVolumeClaimVolumeSource{ClaimName: cdromPVCName, ReadOnly: true},
		})

	case BuilderTypeClone:
		// Resolve source image.
		srcImage, err := client.GetVMImageByDisplayName(cfg.SourceImageNamespace, cfg.SourceImageName)
		if err != nil {
			return nil, fmt.Errorf("source image %q: %w", cfg.SourceImageName, err)
		}
		srcImageID := fmt.Sprintf("%s/%s", srcImage.ObjectMeta.Namespace, srcImage.ObjectMeta.Name)
		ui.Say(fmt.Sprintf("Using source image %q (id: %s)", cfg.SourceImageName, srcImageID))
		storageClass := chooseStorageClass(cfg.StorageClass, srcImage.Status.StorageClassName)
		if storageClass != cfg.StorageClass {
			ui.Say(fmt.Sprintf("Using image storage class %q (from source image)", storageClass))
		}

		// Root disk cloned from source image.
		volClaimTemplates = append(volClaimTemplates, buildVolumeClaimTemplate(
			diskName, cfg.DiskSize, storageClass, srcImageID, cfg.Namespace,
		))
		disks = append(disks, hvclient.DiskTarget{
			Name:      "disk-0",
			BootOrder: 1,
			Disk:      &hvclient.Disk{Bus: "virtio"},
		})
		volumes = append(volumes, hvclient.Volume{
			Name:                  "disk-0",
			PersistentVolumeClaim: &hvclient.PersistentVolumeClaimVolumeSource{ClaimName: diskName},
		})
	default:
		return nil, fmt.Errorf("unsupported builder type %q", builderType)
	}

	// Serialise volumeClaimTemplates annotation.
	vctJSON, err := json.Marshal(volClaimTemplates)
	if err != nil {
		return nil, fmt.Errorf("marshal volumeClaimTemplates: %w", err)
	}

	vm := &hvclient.VirtualMachine{
		TypeMeta: hvclient.TypeMeta{
			APIVersion: "kubevirt.io/v1",
			Kind:       "VirtualMachine",
		},
		ObjectMeta: hvclient.ObjectMeta{
			Name:      s.vmName,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				"harvesterhci.io/creator": "packer",
			},
			Annotations: map[string]string{
				"harvesterhci.io/volumeClaimTemplates": string(vctJSON),
			},
		},
		Spec: hvclient.VirtualMachineSpec{
			RunStrategy: "Once",
			Template: hvclient.VMTemplateSpec{
				ObjectMeta: hvclient.ObjectMeta{
					Labels: map[string]string{
						"harvesterhci.io/vmName": s.vmName,
					},
				},
				Spec: hvclient.VMSpec{
					Domain: hvclient.Domain{
						CPU: hvclient.CPU{
							Cores:   cpu,
							Sockets: 1,
							Threads: 1,
						},
						Resources: hvclient.ResourceRequirements{
							Limits: map[string]string{
								"cpu":    fmt.Sprintf("%d", cpu),
								"memory": memory,
							},
						},
						Devices: hvclient.Devices{
							Disks:      disks,
							Interfaces: ifaces,
						},
					},
					Networks: networks,
					Volumes:  volumes,
				},
			},
		},
	}

	return vm, nil
}

// buildNetworkSpec returns the network and interface specs for the VM.
func buildNetworkSpec(cfg *Config) ([]hvclient.Network, []hvclient.Interface) {
	if cfg.NetworkName != "" {
		// Multus network.
		netName := cfg.NetworkName
		if cfg.NetworkNamespace != "" {
			netName = cfg.NetworkNamespace + "/" + cfg.NetworkName
		}
		return []hvclient.Network{
				{
					Name:   "default",
					Multus: &hvclient.MultusNet{NetworkName: netName, Default: true},
				},
			}, []hvclient.Interface{
				{Name: "default", Model: "virtio", Bridge: struct{}{}},
			}
	}
	// Default pod network.
	return []hvclient.Network{
			{Name: "default", Pod: &hvclient.PodNetwork{}},
		}, []hvclient.Interface{
			{Name: "default", Model: "virtio", Masquerade: struct{}{}},
		}
}

// buildVolumeClaimTemplate constructs a Harvester volumeClaimTemplate entry.
func buildVolumeClaimTemplate(name, size, storageClass, imageID, imageNamespace string) map[string]interface{} {
	annotations := map[string]interface{}{}
	if imageID != "" {
		annotations["harvesterhci.io/imageId"] = imageID
	}

	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":        name,
			"namespace":   imageNamespace,
			"annotations": annotations,
		},
		"spec": map[string]interface{}{
			"accessModes": []string{"ReadWriteMany"},
			"resources": map[string]interface{}{
				"requests": map[string]string{
					"storage": size,
				},
			},
			"volumeMode":       "Block",
			"storageClassName": storageClass,
		},
	}
}

// imageSizeToGi converts a byte count to a Gi quantity string rounded up.
// If the size is unknown, it falls back to 4Gi for common installer ISOs.
func imageSizeToGi(sizeBytes int64) string {
	if sizeBytes <= 0 {
		return "4Gi"
	}
	const gi = 1024 * 1024 * 1024
	sizeGi := int64(math.Ceil(float64(sizeBytes) / float64(gi)))
	if sizeGi < 1 {
		sizeGi = 1
	}
	return fmt.Sprintf("%dGi", sizeGi)
}

func chooseStorageClass(configStorageClass, imageStorageClass string) string {
	if imageStorageClass != "" && (configStorageClass == "" || configStorageClass == hvclient.DefaultStorageClass) {
		return imageStorageClass
	}
	return configStorageClass
}

func chooseISOCDROMStorageClass(rootStorageClass, imageStorageClass string) string {
	if imageStorageClass != "" {
		return imageStorageClass
	}
	return rootStorageClass
}

// randomHex returns a cryptographically random lowercase hex string of n bytes
// (resulting in 2n hex characters).
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use a timestamp-derived value if crypto/rand fails.
		return fmt.Sprintf("%x", time.Now().UnixNano())[:n*2]
	}
	return strings.ToLower(hex.EncodeToString(b))
}
