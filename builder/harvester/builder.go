// Copyright IBM Corp. 2020, 2025
// SPDX-License-Identifier: MPL-2.0

package harvester

import (
	"context"
	"fmt"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"

	hvclient "github.com/hashicorp/packer-plugin-scaffolding/builder/harvester/client"
)

// ISOBuilder builds a Harvester VM image from an installation ISO.
type ISOBuilder struct {
	config Config
	runner multistep.Runner
}

// ConfigSpec returns the HCL2 spec for the ISO builder.
func (b *ISOBuilder) ConfigSpec() hcldec.ObjectSpec {
	return b.config.FlatMapstructure().HCL2Spec()
}

// Prepare validates the ISO builder configuration.
func (b *ISOBuilder) Prepare(raws ...interface{}) ([]string, []string, error) {
	return b.config.Prepare(BuilderTypeISO, raws...)
}

// Run executes the ISO build pipeline.
func (b *ISOBuilder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	client, err := buildClient(&b.config)
	if err != nil {
		return nil, err
	}

	state := new(multistep.BasicStateBag)
	state.Put("hook", hook)
	state.Put("ui", ui)
	state.Put("config", &b.config)
	state.Put("client", client)

	steps := []multistep.Step{
		&StepCreateVM{Config: &b.config},
		&StepWaitForInstance{Config: &b.config},
		&StepBootCommand{Config: &b.config},
		&communicator.StepConnect{
			Config:    &b.config.Config,
			Host:      communicatorHost(b.config.Config),
			SSHConfig: b.config.Config.SSHConfigFunc(),
		},
		new(commonsteps.StepProvision),
		&StepShutdown{Config: &b.config},
		&StepCreateImage{Config: &b.config},
	}

	b.runner = commonsteps.NewRunner(steps, b.config.PackerConfig, ui)
	b.runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	imageName, _ := state.Get("image_name").(string)
	if imageName == "" {
		return nil, fmt.Errorf("no output image was created")
	}

	return &Artifact{
		ImageName:      imageName,
		ImageNamespace: b.config.OutputImageNamespace,
		HarvesterURL:   b.config.HarvesterURL,
		StateData:      map[string]interface{}{"generated_data": state.Get("generated_data")},
	}, nil
}

// CloneBuilder builds a Harvester VM image by cloning an existing image.
type CloneBuilder struct {
	config Config
	runner multistep.Runner
}

// ConfigSpec returns the HCL2 spec for the clone builder.
func (b *CloneBuilder) ConfigSpec() hcldec.ObjectSpec {
	return b.config.FlatMapstructure().HCL2Spec()
}

// Prepare validates the clone builder configuration.
func (b *CloneBuilder) Prepare(raws ...interface{}) ([]string, []string, error) {
	return b.config.Prepare(BuilderTypeClone, raws...)
}

// Run executes the clone build pipeline.
func (b *CloneBuilder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {
	client, err := buildClient(&b.config)
	if err != nil {
		return nil, err
	}

	state := new(multistep.BasicStateBag)
	state.Put("hook", hook)
	state.Put("ui", ui)
	state.Put("config", &b.config)
	state.Put("client", client)

	steps := []multistep.Step{
		&StepCreateVM{Config: &b.config},
		&StepWaitForInstance{Config: &b.config},
		&communicator.StepConnect{
			Config:    &b.config.Config,
			Host:      communicatorHost(b.config.Config),
			SSHConfig: b.config.Config.SSHConfigFunc(),
		},
		new(commonsteps.StepProvision),
		&StepShutdown{Config: &b.config},
		&StepCreateImage{Config: &b.config},
	}

	b.runner = commonsteps.NewRunner(steps, b.config.PackerConfig, ui)
	b.runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	imageName, _ := state.Get("image_name").(string)
	if imageName == "" {
		return nil, fmt.Errorf("no output image was created")
	}

	return &Artifact{
		ImageName:      imageName,
		ImageNamespace: b.config.OutputImageNamespace,
		HarvesterURL:   b.config.HarvesterURL,
		StateData:      map[string]interface{}{"generated_data": state.Get("generated_data")},
	}, nil
}

// buildClient creates the HarvesterClient from the plugin configuration.
// When kubeconfig is set it is always used as the primary credential source;
// explicit harvester_url and token values override the corresponding kubeconfig
// fields when provided.
func buildClient(cfg *Config) (*hvclient.HarvesterClient, error) {
	if cfg.Kubeconfig != "" {
		return hvclient.NewClientFromKubeconfigWithOverrides(
			cfg.Kubeconfig, cfg.Namespace, cfg.HarvesterURL, cfg.Token, cfg.SkipTLSVerify)
	}
	if cfg.HarvesterURL == "" {
		return nil, fmt.Errorf("'harvester_url' must be set when 'kubeconfig' is not provided")
	}
	return hvclient.NewClient(cfg.HarvesterURL, cfg.Namespace, cfg.Token, cfg.SkipTLSVerify), nil
}

// communicatorHost returns the host accessor function for communicator.StepConnect.
func communicatorHost(comm communicator.Config) func(multistep.StateBag) (string, error) {
	return func(state multistep.StateBag) (string, error) {
		// Prefer the IP stored by StepWaitForInstance.
		if ip, ok := state.GetOk("vm_ip"); ok {
			return ip.(string), nil
		}
		// Fall back to the static SSH host from config.
		if comm.SSHHost != "" {
			return comm.SSHHost, nil
		}
		return "", fmt.Errorf("VM IP address is not known yet; ensure StepWaitForInstance ran successfully")
	}
}
