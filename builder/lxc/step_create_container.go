package lxc

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateContainer struct{}

func (s *stepCreateContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	comm := state.Get("communicator").(CommandRunner)
	ctid := state.Get("ctid").(string)

	err := comm.RunCommand(ctx, fmt.Sprintf("pct status %s", ctid), nil, nil)
	if err == nil {
		ui.Say(fmt.Sprintf("Container %s already exists, reusing", ctid))
		state.Put("container_reused", true)
		return multistep.ActionContinue
	}

	ui.Say(fmt.Sprintf("Creating container %s...", ctid))

	unprivileged := "1"
	if !config.Unprivileged {
		unprivileged = "0"
	}

	// Fixed: --rootfs uses config.Storage for pool and config.RootfsSize for size
	cmd := fmt.Sprintf(
		"pct create %s %s --unprivileged %s --features %s --hostname builder-%s --storage %s --rootfs %s:%s --memory %d --cores %d --net0 name=eth0,bridge=%s,ip=dhcp --password '%s'",
		ctid,
		config.Template,
		unprivileged,
		config.Features,
		ctid,
		config.Storage,
		config.Storage,
		config.RootfsSize,
		config.Memory,
		config.Cores,
		config.Bridge,
		config.RootPassword,
	)

	err = comm.RunCommand(ctx, cmd, nil, nil)
	if err != nil {
		state.Put("error", fmt.Errorf("pct create failed: %w", err))
		return multistep.ActionHalt
	}

	state.Put("container_reused", false)
	ui.Say(fmt.Sprintf("Container %s created", ctid))
	return multistep.ActionContinue
}

func (s *stepCreateContainer) Cleanup(state multistep.StateBag) {}
