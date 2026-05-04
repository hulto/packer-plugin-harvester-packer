// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"testing"
	"time"

	hvclient "github.com/hashicorp/packer-plugin-scaffolding/builder/harvester/client"
)

// TestConfigPrepare_ISO tests validation of the ISO builder configuration.
func TestConfigPrepare_ISO(t *testing.T) {
	t.Run("valid_iso_config", func(t *testing.T) {
		cfg := &Config{}
		_, warns, err := cfg.Prepare(BuilderTypeISO, map[string]interface{}{
			"harvester_url":   "https://192.168.1.1:6443",
			"token":           "testtoken",
			"namespace":       "default",
			"iso_image_name":  "ubuntu-22-04-iso",
			"ssh_username":    "ubuntu",
			"ssh_password":    "secret",
			"skip_tls_verify": true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(warns) > 0 {
			t.Logf("warnings: %v", warns)
		}
	})

	t.Run("missing_iso_image_name", func(t *testing.T) {
		cfg := &Config{}
		_, _, err := cfg.Prepare(BuilderTypeISO, map[string]interface{}{
			"harvester_url": "https://192.168.1.1:6443",
			"token":         "testtoken",
			"ssh_username":  "ubuntu",
			"ssh_password":  "secret",
		})
		if err == nil {
			t.Fatal("expected error for missing iso_image_name, got nil")
		}
	})

	t.Run("missing_connection_config", func(t *testing.T) {
		cfg := &Config{}
		_, _, err := cfg.Prepare(BuilderTypeISO, map[string]interface{}{
			"iso_image_name": "ubuntu-22-04-iso",
			"ssh_username":   "ubuntu",
			"ssh_password":   "secret",
		})
		if err == nil {
			t.Fatal("expected error for missing harvester_url/kubeconfig, got nil")
		}
	})
}

// TestConfigPrepare_Clone tests validation of the clone builder configuration.
func TestConfigPrepare_Clone(t *testing.T) {
	t.Run("valid_clone_config", func(t *testing.T) {
		cfg := &Config{}
		_, warns, err := cfg.Prepare(BuilderTypeClone, map[string]interface{}{
			"harvester_url":     "https://192.168.1.1:6443",
			"token":             "testtoken",
			"namespace":         "default",
			"source_image_name": "ubuntu-22-04",
			"ssh_username":      "ubuntu",
			"ssh_password":      "secret",
			"skip_tls_verify":   true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(warns) > 0 {
			t.Logf("warnings: %v", warns)
		}
	})

	t.Run("missing_source_image_name", func(t *testing.T) {
		cfg := &Config{}
		_, _, err := cfg.Prepare(BuilderTypeClone, map[string]interface{}{
			"harvester_url": "https://192.168.1.1:6443",
			"token":         "testtoken",
			"ssh_username":  "ubuntu",
			"ssh_password":  "secret",
		})
		if err == nil {
			t.Fatal("expected error for missing source_image_name, got nil")
		}
	})
}

// TestConfigDefaults verifies that default values are applied correctly.
func TestConfigDefaults(t *testing.T) {
	cfg := &Config{}
	_, _, err := cfg.Prepare(BuilderTypeClone, map[string]interface{}{
		"harvester_url":     "https://192.168.1.1:6443",
		"token":             "testtoken",
		"source_image_name": "ubuntu-22-04",
		"ssh_username":      "ubuntu",
		"ssh_password":      "secret",
		"skip_tls_verify":   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", cfg.Namespace)
	}
	if cfg.CPUCores != 2 {
		t.Errorf("expected cpu_cores 2, got %d", cfg.CPUCores)
	}
	if cfg.MemoryMB != 2048 {
		t.Errorf("expected memory_mb 2048, got %d", cfg.MemoryMB)
	}
	if cfg.DiskSize != "40Gi" {
		t.Errorf("expected disk_size '40Gi', got %q", cfg.DiskSize)
	}
	if cfg.StorageClass != "harvester-longhorn" {
		t.Errorf("expected storage_class 'harvester-longhorn', got %q", cfg.StorageClass)
	}
	if cfg.WaitForInstanceTimeout != 10*time.Minute {
		t.Errorf("expected wait timeout 10m, got %s", cfg.WaitForInstanceTimeout)
	}
	if cfg.ShutdownTimeout != 5*time.Minute {
		t.Errorf("expected shutdown timeout 5m, got %s", cfg.ShutdownTimeout)
	}
	if cfg.BootWait != 10*time.Second {
		t.Errorf("expected boot_wait 10s, got %s", cfg.BootWait)
	}
}

// TestConfigNamespacePropagation tests that image namespace defaults inherit
// from the main namespace.
func TestConfigNamespacePropagation(t *testing.T) {
	cfg := &Config{}
	_, _, err := cfg.Prepare(BuilderTypeISO, map[string]interface{}{
		"harvester_url":   "https://192.168.1.1:6443",
		"token":           "testtoken",
		"namespace":       "mynamespace",
		"iso_image_name":  "myiso",
		"ssh_username":    "ubuntu",
		"ssh_password":    "secret",
		"skip_tls_verify": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ISOImageNamespace != "mynamespace" {
		t.Errorf("expected iso_image_namespace 'mynamespace', got %q", cfg.ISOImageNamespace)
	}
	if cfg.OutputImageNamespace != "mynamespace" {
		t.Errorf("expected output_image_namespace 'mynamespace', got %q", cfg.OutputImageNamespace)
	}
}

func TestEffectiveBuilderType(t *testing.T) {
	t.Run("uses_explicit_builder_type", func(t *testing.T) {
		cfg := &Config{builderType: BuilderTypeISO}
		got, err := cfg.effectiveBuilderType()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != BuilderTypeISO {
			t.Fatalf("expected %q, got %q", BuilderTypeISO, got)
		}
	})

	t.Run("infers_iso_from_config", func(t *testing.T) {
		cfg := &Config{ISOImageName: "ubuntu-24.iso"}
		got, err := cfg.effectiveBuilderType()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != BuilderTypeISO {
			t.Fatalf("expected %q, got %q", BuilderTypeISO, got)
		}
	})

	t.Run("infers_clone_from_config", func(t *testing.T) {
		cfg := &Config{SourceImageName: "ubuntu-24-golden"}
		got, err := cfg.effectiveBuilderType()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != BuilderTypeClone {
			t.Fatalf("expected %q, got %q", BuilderTypeClone, got)
		}
	})

	t.Run("errors_when_ambiguous", func(t *testing.T) {
		cfg := &Config{ISOImageName: "ubuntu-24.iso", SourceImageName: "golden"}
		if _, err := cfg.effectiveBuilderType(); err == nil {
			t.Fatal("expected error when both iso_image_name and source_image_name are set")
		}
	})

	t.Run("errors_when_missing", func(t *testing.T) {
		cfg := &Config{}
		if _, err := cfg.effectiveBuilderType(); err == nil {
			t.Fatal("expected error when no builder type can be inferred")
		}
	})
}

func TestBuildVolumeClaimTemplate_UsesProvidedNamespace(t *testing.T) {
	vct := buildVolumeClaimTemplate("cdrom", "1Gi", "harvester-longhorn", "harvester-public/image-1", "hulto")

	metaRaw, ok := vct["metadata"]
	if !ok {
		t.Fatal("expected metadata in volume claim template")
	}
	meta, ok := metaRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata map, got %T", metaRaw)
	}
	if got, _ := meta["namespace"].(string); got != "hulto" {
		t.Fatalf("expected namespace hulto, got %q", got)
	}

	specRaw, ok := vct["spec"]
	if !ok {
		t.Fatal("expected spec in volume claim template")
	}
	spec, ok := specRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("expected spec map, got %T", specRaw)
	}
	modes, ok := spec["accessModes"].([]string)
	if !ok {
		t.Fatalf("expected accessModes []string, got %T", spec["accessModes"])
	}
	if len(modes) != 1 || modes[0] != "ReadWriteMany" {
		t.Fatalf("expected accessModes [ReadWriteMany], got %v", modes)
	}
}

func TestImageSizeToGi(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{name: "unknown", in: 0, want: "4Gi"},
		{name: "one_gi", in: 1024 * 1024 * 1024, want: "1Gi"},
		{name: "round_up", in: 3*1024*1024*1024 + 1, want: "4Gi"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := imageSizeToGi(tc.in); got != tc.want {
				t.Fatalf("imageSizeToGi(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestChooseStorageClass(t *testing.T) {
	tests := []struct {
		name     string
		configSC string
		imageSC  string
		want     string
	}{
		{name: "prefer_image_when_default", configSC: hvclient.DefaultStorageClass, imageSC: "duplicated", want: "duplicated"},
		{name: "prefer_image_when_empty", configSC: "", imageSC: "duplicated", want: "duplicated"},
		{name: "keep_explicit_config", configSC: "fast-ssd", imageSC: "duplicated", want: "fast-ssd"},
		{name: "keep_config_when_image_empty", configSC: "fast-ssd", imageSC: "", want: "fast-ssd"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := chooseStorageClass(tc.configSC, tc.imageSC); got != tc.want {
				t.Fatalf("chooseStorageClass(%q, %q) = %q, want %q", tc.configSC, tc.imageSC, got, tc.want)
			}
		})
	}
}

func TestChooseISOCDROMStorageClass(t *testing.T) {
	tests := []struct {
		name   string
		rootSC string
		imgSC  string
		want   string
	}{
		{name: "prefer_image_sc", rootSC: "duplicated", imgSC: "longhorn-image-57448", want: "longhorn-image-57448"},
		{name: "fallback_to_root", rootSC: "duplicated", imgSC: "", want: "duplicated"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := chooseISOCDROMStorageClass(tc.rootSC, tc.imgSC); got != tc.want {
				t.Fatalf("chooseISOCDROMStorageClass(%q, %q) = %q, want %q", tc.rootSC, tc.imgSC, got, tc.want)
			}
		})
	}
}

func TestConfigPrepare_CloudInitNoCloud(t *testing.T) {
	t.Run("accepts_cloud_init_without_http", func(t *testing.T) {
		cfg := &Config{}
		_, _, err := cfg.Prepare(BuilderTypeISO, map[string]interface{}{
			"harvester_url":        "https://192.168.1.1:6443",
			"token":                "testtoken",
			"iso_image_name":       "ubuntu-24-iso",
			"ssh_username":         "ubuntu",
			"ssh_password":         "secret",
			"cloud_init_user_data": "#cloud-config\nusers: []\n",
			"skip_tls_verify":      true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects_partial_cloud_init_without_user_data", func(t *testing.T) {
		cfg := &Config{}
		_, _, err := cfg.Prepare(BuilderTypeISO, map[string]interface{}{
			"harvester_url":           "https://192.168.1.1:6443",
			"token":                   "testtoken",
			"iso_image_name":          "ubuntu-24-iso",
			"ssh_username":            "ubuntu",
			"ssh_password":            "secret",
			"cloud_init_network_data": "version: 2\n",
			"skip_tls_verify":         true,
		})
		if err == nil {
			t.Fatal("expected error for missing cloud_init_user_data")
		}
	})

	t.Run("rejects_http_directory_with_cloud_init", func(t *testing.T) {
		cfg := &Config{}
		_, _, err := cfg.Prepare(BuilderTypeISO, map[string]interface{}{
			"harvester_url":        "https://192.168.1.1:6443",
			"token":                "testtoken",
			"iso_image_name":       "ubuntu-24-iso",
			"ssh_username":         "ubuntu",
			"ssh_password":         "secret",
			"cloud_init_user_data": "#cloud-config\nusers: []\n",
			"http_directory":       "./http",
			"skip_tls_verify":      true,
		})
		if err == nil {
			t.Fatal("expected error for mixing http_directory and cloud-init NoCloud")
		}
	})

	t.Run("rejects_http_template_vars_with_cloud_init", func(t *testing.T) {
		cfg := &Config{}
		_, _, err := cfg.Prepare(BuilderTypeISO, map[string]interface{}{
			"harvester_url":        "https://192.168.1.1:6443",
			"token":                "testtoken",
			"iso_image_name":       "ubuntu-24-iso",
			"ssh_username":         "ubuntu",
			"ssh_password":         "secret",
			"cloud_init_user_data": "#cloud-config\nusers: []\n",
			"boot_command": []string{
				" autoinstall ds=nocloud-net;s=http://{{.HTTPIP}}:{{.HTTPPort}}/",
			},
			"skip_tls_verify": true,
		})
		if err == nil {
			t.Fatal("expected error for HTTP template variables with cloud-init NoCloud")
		}
	})
}
