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
	ctid, err := fetchNextCTID(ctx, comm)
	if err != nil {
		state.Put("error", err)
		return multistep.ActionHalt
	}

	state.Put("ctid", ctid)
	ui.Say(fmt.Sprintf("Using CTID: %s", ctid))
	return multistep.ActionContinue
}

func (s *stepGetCTID) Cleanup(state multistep.StateBag) {}

// fetchNextCTID asks Proxmox for the next available CTID. Note this is a
// point-in-time query, not a reservation: when multiple builds target the
// same host concurrently, they can race and get the same ID back. Callers
// that auto-assign a CTID should be prepared to retry with a freshly
// fetched ID if container creation subsequently fails because the ID was
// claimed by a concurrent build in the meantime (see stepCreateContainer).
func fetchNextCTID(ctx context.Context, comm CommandRunner) (string, error) {
	var stdout bytes.Buffer
	if err := comm.RunCommand(ctx, "pvesh get /cluster/nextid", &stdout, nil); err != nil {
		return "", fmt.Errorf("failed to get next CTID: %w", err)
	}

	ctid := strings.TrimSpace(stdout.String())
	if ctid == "" {
		return "", fmt.Errorf("got empty CTID from Proxmox")
	}
	return ctid, nil
}
