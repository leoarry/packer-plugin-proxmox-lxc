package lxc

import (
	"context"
	"fmt"

	hcldec "github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/steps"
)

// Builder implements packersdk.Builder for Proxmox LXC template creation.
type Builder struct {
	config config.Config
	runner multistep.Runner
	steps  []multistep.Step // injectable for testing
}

// Ensure Builder implements packersdk.Builder.
var _ packersdk.Builder = &Builder{}

// ConfigSpec returns the HCL2 specification for this builder.
func (b *Builder) ConfigSpec() hcldec.ObjectSpec {
	return b.config.FlatMapstructure().HCL2Spec()
}

// Prepare decodes and validates the configuration.
func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	warnings, generated, err := b.config.Prepare(raws...)
	if err != nil {
		return nil, nil, err
	}
	return generated, warnings, nil
}

// setSteps sets the steps for the builder (for testing).
func (b *Builder) setSteps(steps []multistep.Step) {
	b.steps = steps
}

// Run executes the build process.
func (b *Builder) Run(ctx context.Context, ui packersdk.Ui, hook packersdk.Hook) (packersdk.Artifact, error) {
	// Use injectable steps if set (for testing), otherwise use defaults.
	pipeline := b.steps
	if pipeline == nil {
		pipeline = []multistep.Step{
			&steps.Connect{},
			&steps.GetCTID{},
			&steps.CreateContainer{},
			&steps.MergeConfig{},
			&steps.StartContainer{},
			&steps.SetupContainerComm{},
			&commonsteps.StepProvision{},
		}
		if b.config.BackupMethod == "template" {
			pipeline = append(pipeline, &steps.CreateTemplate{})
		} else {
			pipeline = append(pipeline, &steps.BackupContainer{}, &steps.DestroyContainer{})
		}
	}

	// Setup state bag.
	state := new(multistep.BasicStateBag)
	state.Put(steps.StateConfig, &b.config)
	state.Put(steps.StateHook, hook)
	state.Put(steps.StateUI, ui)

	// Run the steps.
	b.runner = commonsteps.NewRunner(pipeline, b.config.PackerConfig, ui)
	b.runner.Run(ctx, state)

	// Check for errors.
	if rawErr, ok := state.GetOk(steps.StateError); ok {
		return nil, rawErr.(error)
	}

	// Get artifact data from state.
	ctid, _ := state.Get(steps.StateCTID).(string)

	if b.config.BackupMethod == "template" {
		templated, _ := state.Get(steps.StateContainerTemplated).(bool)
		if !templated {
			return nil, fmt.Errorf("container was not converted to a CT template")
		}
		return &Artifact{
			Method: "template",
			CTID:   ctid,
			StateData: map[string]interface{}{
				"ctid": ctid,
			},
		}, nil
	}

	backupPath, _ := state.Get(steps.StateBackupPath).(string)
	backupName, _ := state.Get(steps.StateBackupName).(string)

	if backupPath == "" {
		return nil, fmt.Errorf("no backup path found in state")
	}

	// Return artifact.
	return &Artifact{
		Method:     "vzdump",
		BackupPath: backupPath,
		StateData: map[string]interface{}{
			"backup_name": backupName,
			"ctid":        ctid,
		},
	}, nil
}
