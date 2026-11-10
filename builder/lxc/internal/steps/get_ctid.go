package steps

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

type GetCTID struct{}

func (s *GetCTID) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get(StateUI).(packersdk.Ui)
	cfg := state.Get(StateConfig).(*config.Config)

	if cfg.CTID != "" {
		ui.Say(fmt.Sprintf("Using configured CTID: %s", cfg.CTID))
		state.Put(StateCTID, cfg.CTID)
		return multistep.ActionContinue
	}

	ui.Say("Getting next available CTID...")

	comm := state.Get(StateCommunicator).(remote.CommandRunner)
	ctid, err := fetchNextCTID(ctx, comm)
	if err != nil {
		state.Put(StateError, err)
		return multistep.ActionHalt
	}

	state.Put(StateCTID, ctid)
	ui.Say(fmt.Sprintf("Using CTID: %s", ctid))
	return multistep.ActionContinue
}

func (s *GetCTID) Cleanup(state multistep.StateBag) {}

// fetchNextCTID asks Proxmox for the next available CTID. Note this is a
// point-in-time query, not a reservation: when multiple builds target the
// same host concurrently, they can race and get the same ID back. Callers
// that auto-assign a CTID should be prepared to retry with a freshly
// fetched ID if container creation subsequently fails because the ID was
// claimed by a concurrent build in the meantime (see CreateContainer).
func fetchNextCTID(ctx context.Context, comm remote.CommandRunner) (string, error) {
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
