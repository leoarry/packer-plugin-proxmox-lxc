package steps

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

type DestroyContainer struct{}

func (s *DestroyContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get(StateUI).(packersdk.Ui)
	ctid := state.Get(StateCTID).(string)
	comm := state.Get(StateProxmoxComm).(remote.CommandRunner)

	ui.Say(fmt.Sprintf("Stopping container %s...", ctid))
	err := comm.RunCommand(ctx, fmt.Sprintf("pct stop %s", ctid), nil, nil)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to stop container: %v", err))
	}

	ui.Say(fmt.Sprintf("Destroying container %s...", ctid))
	err = comm.RunCommand(ctx, fmt.Sprintf("pct destroy %s --purge", ctid), nil, nil)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to destroy container: %v", err))
	} else {
		state.Put(StateContainerDestroyed, true)
	}

	_ = comm.RunCommand(ctx, fmt.Sprintf("rm -f /tmp/vzdump-lxc-%s-*.log 2>/dev/null || true", ctid), nil, nil)
	ui.Say(fmt.Sprintf("Container %s destroyed", ctid))
	return multistep.ActionContinue
}

func (s *DestroyContainer) Cleanup(state multistep.StateBag) {}
