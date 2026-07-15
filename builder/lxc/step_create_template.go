package lxc

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// stepCreateTemplate converts the provisioned container into a Proxmox CT
// template (via `pct template`), used as an alternative to the vzdump
// backup flow. The container itself becomes the artifact and is left in
// place on the Proxmox host rather than being destroyed.
type stepCreateTemplate struct{}

func (s *stepCreateTemplate) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	ctid := state.Get("ctid").(string)
	comm := state.Get("proxmox_comm").(CommandRunner)

	ui.Say(fmt.Sprintf("Stopping container %s...", ctid))
	if err := comm.RunCommand(ctx, fmt.Sprintf("pct stop %s", ctid), nil, nil); err != nil {
		state.Put("error", fmt.Errorf("failed to stop container: %w", err))
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Converting container %s into a Proxmox CT template...", ctid))
	if err := comm.RunCommand(ctx, fmt.Sprintf("pct template %s", ctid), nil, nil); err != nil {
		state.Put("error", fmt.Errorf("pct template failed: %w", err))
		return multistep.ActionHalt
	}

	state.Put("container_templated", true)
	ui.Say(fmt.Sprintf("Container %s converted to a CT template", ctid))
	return multistep.ActionContinue
}

func (s *stepCreateTemplate) Cleanup(state multistep.StateBag) {}
