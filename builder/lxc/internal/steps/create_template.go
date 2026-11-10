package steps

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

// CreateTemplate converts the provisioned container into a Proxmox CT
// template (via `pct template`), used as an alternative to the vzdump
// backup flow. The container itself becomes the artifact and is left in
// place on the Proxmox host rather than being destroyed.
type CreateTemplate struct{}

func (s *CreateTemplate) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get(StateUI).(packersdk.Ui)
	ctid := state.Get(StateCTID).(string)
	comm := state.Get(StateProxmoxComm).(remote.CommandRunner)

	ui.Say(fmt.Sprintf("Stopping container %s...", ctid))
	if err := comm.RunCommand(ctx, fmt.Sprintf("pct stop %s", ctid), nil, nil); err != nil {
		state.Put(StateError, fmt.Errorf("failed to stop container: %w", err))
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Converting container %s into a Proxmox CT template...", ctid))
	if err := comm.RunCommand(ctx, fmt.Sprintf("pct template %s", ctid), nil, nil); err != nil {
		state.Put(StateError, fmt.Errorf("pct template failed: %w", err))
		return multistep.ActionHalt
	}

	state.Put(StateContainerTemplated, true)
	ui.Say(fmt.Sprintf("Container %s converted to a CT template", ctid))
	return multistep.ActionContinue
}

func (s *CreateTemplate) Cleanup(state multistep.StateBag) {}
