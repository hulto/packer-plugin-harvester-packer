// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"testing"
	"time"
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
