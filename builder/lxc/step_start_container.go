package lxc

import (
	"context"
	"fmt"
	"time"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

type stepStartContainer struct{}

func (s *stepStartContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	ctid := state.Get("ctid").(string)
	comm := state.Get("communicator").(CommandRunner)

	ui.Say(fmt.Sprintf("Starting container %s...", ctid))

	_, err := comm.RunCommand(ctx, fmt.Sprintf("pct start %s", ctid))
	if err != nil {
		state.Put("error", fmt.Errorf("failed to start container: %w", err))
		return multistep.ActionHalt
	}

	ui.Say("Waiting for container to be ready...")
	time.Sleep(3 * time.Second)

	ui.Say(fmt.Sprintf("Container %s started", ctid))
	return multistep.ActionContinue
}

func (s *stepStartContainer) Cleanup(state multistep.StateBag) {}
