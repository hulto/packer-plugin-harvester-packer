// Copyright IBM Corp. 2020, 2026
// SPDX-License-Identifier: MPL-2.0

package harvester

import "testing"

func TestAppendCloudInitNoCloudDiskAndVolume(t *testing.T) {
	t.Run("applies_defaults_for_meta_data", func(t *testing.T) {
		cfg := &Config{
			CloudInitUserData: "#cloud-config\nusers: []\n",
		}

		disks, volumes := appendCloudInitNoCloudDiskAndVolume(nil, nil, cfg, "packer-vm")
		if len(disks) != 1 {
			t.Fatalf("expected 1 disk, got %d", len(disks))
		}
		if len(volumes) != 1 {
			t.Fatalf("expected 1 volume, got %d", len(volumes))
		}
		if disks[0].Name != "cloudinitdisk" || disks[0].Disk == nil || disks[0].Disk.Bus != "virtio" {
			t.Fatalf("unexpected cloud-init disk %+v", disks[0])
		}
		if volumes[0].Name != "cloudinitdisk" || volumes[0].CloudInitNoCloud == nil {
			t.Fatalf("unexpected cloud-init volume %+v", volumes[0])
		}
		if volumes[0].CloudInitNoCloud.UserData != cfg.CloudInitUserData {
			t.Fatalf("unexpected userData: %q", volumes[0].CloudInitNoCloud.UserData)
		}
		if want := "instance-id: packer-vm\nlocal-hostname: packer-vm\n"; volumes[0].CloudInitNoCloud.MetaData != want {
			t.Fatalf("unexpected default metaData: %q", volumes[0].CloudInitNoCloud.MetaData)
		}
	})

	t.Run("uses_explicit_meta_and_network_data", func(t *testing.T) {
		cfg := &Config{
			CloudInitUserData:    "#cloud-config\nusers: []\n",
			CloudInitMetaData:    "instance-id: custom\n",
			CloudInitNetworkData: "version: 2\n",
		}

		_, volumes := appendCloudInitNoCloudDiskAndVolume(nil, nil, cfg, "ignored")
		if got := volumes[0].CloudInitNoCloud.MetaData; got != cfg.CloudInitMetaData {
			t.Fatalf("expected explicit metaData, got %q", got)
		}
		if got := volumes[0].CloudInitNoCloud.NetworkData; got != cfg.CloudInitNetworkData {
			t.Fatalf("expected networkData %q, got %q", cfg.CloudInitNetworkData, got)
		}
	})
}
