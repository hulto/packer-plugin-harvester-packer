// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package harvester

import (
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

// BuilderType is the type of builder (iso or clone).
type BuilderType string

const (
	// BuilderTypeISO creates a VM from an ISO image.
	BuilderTypeISO BuilderType = "iso"
	// BuilderTypeClone creates a VM by cloning an existing image.
	BuilderTypeClone BuilderType = "clone"
)

// Config contains all configuration fields for the Harvester packer builder.
type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	communicator.Config `mapstructure:",squash"`

	// --- Harvester connection ---

	// HarvesterURL is the base URL of the Harvester API server,
	// e.g. "https://192.168.1.100:6443".
	HarvesterURL string `mapstructure:"harvester_url"`

	// Kubeconfig is the path to a kubeconfig file. If set, HarvesterURL and
	// Token are derived from it unless explicitly overridden.
	Kubeconfig string `mapstructure:"kubeconfig"`

	// Namespace is the Kubernetes namespace where VM resources are created.
	// Defaults to "default".
	Namespace string `mapstructure:"namespace"`

	// Token is a Kubernetes service-account bearer token used to authenticate
	// against the Harvester API.
	Token string `mapstructure:"token"`

	// SkipTLSVerify disables TLS certificate validation when communicating
	// with the Harvester API. Not recommended for production.
	SkipTLSVerify bool `mapstructure:"skip_tls_verify"`

	// --- VM configuration ---

	// VMName is the name assigned to the temporary build VM.
	// A unique name is generated when omitted.
	VMName string `mapstructure:"vm_name"`

	// CPUCores is the number of vCPU cores for the build VM. Defaults to 2.
	CPUCores int `mapstructure:"cpu_cores"`

	// MemoryMB is the amount of RAM for the build VM in megabytes. Defaults to 2048.
	MemoryMB int `mapstructure:"memory_mb"`

	// DiskSize is the root disk size, e.g. "40Gi". Defaults to "40Gi".
	DiskSize string `mapstructure:"disk_size"`

	// StorageClass is the Kubernetes StorageClass for persistent volumes.
	// Defaults to "harvester-longhorn".
	StorageClass string `mapstructure:"storage_class"`

	// --- Network ---

	// NetworkName is the name of the Harvester network attachment or pod
	// network to attach to the VM. Defaults to the Kubernetes pod network.
	NetworkName string `mapstructure:"network_name"`

	// NetworkNamespace is the namespace of the NetworkAttachmentDefinition.
	// Only needed when NetworkName refers to a Multus network.
	NetworkNamespace string `mapstructure:"network_namespace"`

	// --- ISO builder ---

	// ISOImageName is the name of the VirtualMachineImage (in Harvester) that
	// contains the installation ISO. Required for the ISO builder type.
	ISOImageName string `mapstructure:"iso_image_name"`

	// ISOImageNamespace is the namespace of the ISO VirtualMachineImage.
	// Defaults to Namespace when omitted.
	ISOImageNamespace string `mapstructure:"iso_image_namespace"`

	// --- Clone builder ---

	// SourceImageName is the name of the VirtualMachineImage (in Harvester)
	// to clone as the root disk. Required for the clone builder type.
	SourceImageName string `mapstructure:"source_image_name"`

	// SourceImageNamespace is the namespace of the source VirtualMachineImage.
	// Defaults to Namespace when omitted.
	SourceImageNamespace string `mapstructure:"source_image_namespace"`

	// --- Output ---

	// OutputImageName is the display name of the VirtualMachineImage produced
	// by the build. Defaults to "packer-<VMName>".
	OutputImageName string `mapstructure:"output_image_name"`

	// OutputImageNamespace is the namespace for the output VirtualMachineImage.
	// Defaults to Namespace.
	OutputImageNamespace string `mapstructure:"output_image_namespace"`

	// --- Boot ---

	// BootCommand is a list of commands to send to the VM via the VNC console
	// immediately after the VM first boots.
	BootCommand []string `mapstructure:"boot_command"`

	// BootWait is how long to wait after the VM starts before sending boot
	// commands. Defaults to 10 seconds.
	BootWait time.Duration `mapstructure:"boot_wait"`

	// --- HTTP server (for preseed/cloud-init delivery during ISO installs) ---

	// HTTPDir is a directory that will be served over HTTP during the build so
	// that boot commands can reference {{.HTTPIP}} and {{.HTTPPort}}.
	HTTPDir string `mapstructure:"http_directory"`

	// HTTPPortMin and HTTPPortMax define the port range for the local HTTP
	// server. Defaults to 8000–9000.
	HTTPPortMin int `mapstructure:"http_port_min"`
	HTTPPortMax int `mapstructure:"http_port_max"`

	// WaitForInstanceTimeout is the maximum time to wait for the VM to reach
	// Running state. Defaults to 10 minutes.
	WaitForInstanceTimeout time.Duration `mapstructure:"wait_for_instance_timeout"`

	// ShutdownCommand is an optional shell command to run inside the guest to
	// initiate shutdown. When empty a graceful stop is requested via the API.
	ShutdownCommand string `mapstructure:"shutdown_command"`

	// ShutdownTimeout is the time to wait for the VM to stop after
	// issuing the shutdown command. Defaults to 5 minutes.
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`

	// builderType is set by the registered builder name (iso or clone).
	builderType BuilderType

	ctx interpolate.Context
}

// Prepare validates and normalises the configuration.
func (c *Config) Prepare(builderType BuilderType, raws ...interface{}) ([]string, []string, error) {
	c.builderType = builderType

	err := config.Decode(c, &config.DecodeOpts{
		PluginType:         "packer.builder.harvester." + string(builderType),
		Interpolate:        true,
		InterpolateContext: &c.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{"boot_command"},
		},
	}, raws...)
	if err != nil {
		return nil, nil, err
	}

	var errs *packer_errs
	var warnings []string

	// Defaults.
	if c.Namespace == "" {
		c.Namespace = "default"
	}
	if c.CPUCores == 0 {
		c.CPUCores = 2
	}
	if c.MemoryMB == 0 {
		c.MemoryMB = 2048
	}
	if c.DiskSize == "" {
		c.DiskSize = "40Gi"
	}
	if c.StorageClass == "" {
		c.StorageClass = "harvester-longhorn"
	}
	if c.BootWait == 0 {
		c.BootWait = 10 * time.Second
	}
	if c.WaitForInstanceTimeout == 0 {
		c.WaitForInstanceTimeout = 10 * time.Minute
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = 5 * time.Minute
	}
	if c.HTTPPortMin == 0 {
		c.HTTPPortMin = 8000
	}
	if c.HTTPPortMax == 0 {
		c.HTTPPortMax = 9000
	}

	// Default namespace fall-through for image namespaces.
	if c.ISOImageNamespace == "" {
		c.ISOImageNamespace = c.Namespace
	}
	if c.SourceImageNamespace == "" {
		c.SourceImageNamespace = c.Namespace
	}
	if c.OutputImageNamespace == "" {
		c.OutputImageNamespace = c.Namespace
	}

	// Connection validation.
	if c.HarvesterURL == "" && c.Kubeconfig == "" {
		errs = appendErr(errs, errors.New("one of 'harvester_url' or 'kubeconfig' must be set"))
	}
	if c.HarvesterURL != "" && c.Token == "" && c.Kubeconfig == "" {
		warnings = append(warnings, "'token' is not set; ensure your Harvester API accepts unauthenticated requests (not recommended)")
	}

	// Type-specific validation.
	switch builderType {
	case BuilderTypeISO:
		if c.ISOImageName == "" {
			errs = appendErr(errs, errors.New("'iso_image_name' is required for the ISO builder"))
		}
	case BuilderTypeClone:
		if c.SourceImageName == "" {
			errs = appendErr(errs, errors.New("'source_image_name' is required for the clone builder"))
		}
	default:
		errs = appendErr(errs, fmt.Errorf("unknown builder type %q", builderType))
	}

	// SSH / WinRM communicator defaults.
	if c.Config.Type == "" {
		c.Config.Type = "ssh"
	}
	if c.Config.SSH.SSHTimeout == 0 {
		c.Config.SSH.SSHTimeout = 10 * time.Minute
	}

	if commErrs := c.Config.Prepare(&c.ctx); len(commErrs) > 0 {
		for _, e := range commErrs {
			errs = appendErr(errs, e)
		}
	}

	if errs != nil && len(errs.errors) > 0 {
		return nil, warnings, errs.toErr()
	}
	return nil, warnings, nil
}

// --- small error helper ---

type packer_errs struct {
	errors []error
}

func appendErr(pe *packer_errs, err error) *packer_errs {
	if pe == nil {
		pe = &packer_errs{}
	}
	pe.errors = append(pe.errors, err)
	return pe
}

func (pe *packer_errs) toErr() error {
	if pe == nil || len(pe.errors) == 0 {
		return nil
	}
	msgs := make([]string, len(pe.errors))
	for i, e := range pe.errors {
		msgs[i] = e.Error()
	}
	combined := ""
	for i, m := range msgs {
		if i > 0 {
			combined += "; "
		}
		combined += m
	}
	return errors.New(combined)
}
