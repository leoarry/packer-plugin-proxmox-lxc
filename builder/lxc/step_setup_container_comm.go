package lxc

import (
	"context"
	"fmt"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

type stepSetupContainerComm struct{}

func (s *stepSetupContainerComm) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	ctid := state.Get("ctid").(string)
	parentComm := state.Get("communicator").(*sshCommunicator)

	ui.Say(fmt.Sprintf("Setting up communicator for container %s...", ctid))

	state.Put("proxmox_comm", parentComm)

	comm := &pctExecCommunicator{
		ctid:   ctid,
		parent: parentComm,
	}
	state.Put("communicator", comm)
	ui.Say(fmt.Sprintf("Communicator for container %s ready", ctid))
	return multistep.ActionContinue
}

func (s *stepSetupContainerComm) Cleanup(state multistep.StateBag) {}
