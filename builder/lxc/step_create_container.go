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
		"pct create %s %s --unprivileged %s --features %s --hostname builder-%s --storage %s --rootfs %s:%s --memory %d --cores %d --net0 %s --password '%s'",
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
		config.NetworkConfig(),
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

func (s *stepCreateContainer) Cleanup(state multistep.StateBag) {
	// If container was already destroyed by stepDestroyContainer, skip cleanup.
	if _, ok := state.GetOk("container_destroyed"); ok {
		return
	}

	// If container was reused (existed before our build), don't destroy it.
	if reused, ok := state.GetOk("container_reused"); ok && reused.(bool) {
		return
	}

	ctid, ok := state.GetOk("ctid")
	if !ok {
		return
	}
	ctidStr := ctid.(string)

	// Get the command runner for the Proxmox host.
	// After stepSetupContainerComm, the host communicator is saved as "proxmox_comm".
	// Before that, the communicator in "communicator" is the sshCommunicator.
	var cmdRunner CommandRunner
	if comm, ok := state.GetOk("proxmox_comm"); ok {
		cmdRunner = comm.(CommandRunner)
	} else if comm, ok := state.GetOk("communicator"); ok {
		if cr, ok := comm.(CommandRunner); ok {
			cmdRunner = cr
		}
	}
	if cmdRunner == nil {
		return
	}

	if ui, ok := state.GetOk("ui"); ok {
		ui.(packersdk.Ui).Say(fmt.Sprintf("Cleaning up container %s due to failure...", ctidStr))
	}

	// Best-effort cleanup: stop and destroy the container on the Proxmox host.
	_ = cmdRunner.RunCommand(context.Background(), fmt.Sprintf("pct stop %s || true", ctidStr), nil, nil)
	_ = cmdRunner.RunCommand(context.Background(), fmt.Sprintf("pct destroy %s --purge", ctidStr), nil, nil)
	_ = cmdRunner.RunCommand(context.Background(), fmt.Sprintf("rm -f /tmp/vzdump-lxc-%s-*.log 2>/dev/null || true", ctidStr), nil, nil)
}
