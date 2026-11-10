package steps

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

type StartContainer struct{}

func (s *StartContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get(StateUI).(packersdk.Ui)
	ctid := state.Get(StateCTID).(string)
	comm := state.Get(StateCommunicator).(remote.CommandRunner)

	ui.Say(fmt.Sprintf("Starting container %s...", ctid))

	err := comm.RunCommand(ctx, fmt.Sprintf("pct start %s", ctid), nil, nil)
	if err != nil {
		state.Put(StateError, fmt.Errorf("failed to start container: %w", err))
		return multistep.ActionHalt
	}

	ui.Say("Waiting for container to be ready...")
	time.Sleep(3 * time.Second)

	ui.Say(fmt.Sprintf("Container %s started", ctid))
	return multistep.ActionContinue
}

func (s *StartContainer) Cleanup(state multistep.StateBag) {}
