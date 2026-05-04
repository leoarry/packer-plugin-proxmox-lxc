package lxc

import (
	"context"
	"fmt"
	"strings"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

type stepGetCTID struct{}

func (s *stepGetCTID) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)

	if config.CTID != "" {
		ui.Say(fmt.Sprintf("Using configured CTID: %s", config.CTID))
		state.Put("ctid", config.CTID)
		return multistep.ActionContinue
	}

	ui.Say("Getting next available CTID...")

	comm := state.Get("communicator").(CommandRunner)
	result, err := comm.RunCommand(ctx, "pvesh get /cluster/nextid")
	if err != nil {
		state.Put("error", fmt.Errorf("failed to get next CTID: %w", err))
		return multistep.ActionHalt
	}

	ctid := strings.TrimSpace(result)
	if ctid == "" {
		state.Put("error", fmt.Errorf("got empty CTID from Proxmox"))
		return multistep.ActionHalt
	}

	state.Put("ctid", ctid)
	ui.Say(fmt.Sprintf("Using CTID: %s", ctid))
	return multistep.ActionContinue
}

func (s *stepGetCTID) Cleanup(state multistep.StateBag) {}
