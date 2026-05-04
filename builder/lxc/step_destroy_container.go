package lxc

import (
	"context"
	"fmt"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

type stepDestroyContainer struct{}

func (s *stepDestroyContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	ctid := state.Get("ctid").(string)
	comm := state.Get("proxmox_comm").(CommandRunner)

	ui.Say(fmt.Sprintf("Stopping container %s...", ctid))
	_, err := comm.RunCommand(ctx, fmt.Sprintf("pct stop %s", ctid))
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to stop container: %v", err))
	}

	ui.Say(fmt.Sprintf("Destroying container %s...", ctid))
	_, err = comm.RunCommand(ctx, fmt.Sprintf("pct destroy %s --purge", ctid))
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to destroy container: %v", err))
	}

	_, _ = comm.RunCommand(ctx, fmt.Sprintf("rm -f /tmp/vzdump-lxc-%s-*.log 2>/dev/null || true", ctid))
	ui.Say(fmt.Sprintf("Container %s destroyed", ctid))
	return multistep.ActionContinue
}

func (s *stepDestroyContainer) Cleanup(state multistep.StateBag) {}
