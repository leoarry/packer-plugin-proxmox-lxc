package lxc

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
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
	var stdout bytes.Buffer
	err := comm.RunCommand(ctx, "pvesh get /cluster/nextid", &stdout, nil)
	if err != nil {
		state.Put("error", fmt.Errorf("failed to get next CTID: %w", err))
		return multistep.ActionHalt
	}

	ctid := strings.TrimSpace(stdout.String())
	if ctid == "" {
		state.Put("error", fmt.Errorf("got empty CTID from Proxmox"))
		return multistep.ActionHalt
	}

	state.Put("ctid", ctid)
	ui.Say(fmt.Sprintf("Using CTID: %s", ctid))
	return multistep.ActionContinue
}

func (s *stepGetCTID) Cleanup(state multistep.StateBag) {}
