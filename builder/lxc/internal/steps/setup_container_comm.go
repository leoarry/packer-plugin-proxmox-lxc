package steps

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

type SetupContainerComm struct{}

func (s *SetupContainerComm) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get(StateUI).(packersdk.Ui)
	ctid := state.Get(StateCTID).(string)
	parentComm := state.Get(StateCommunicator).(*remote.SSHCommunicator)

	ui.Say(fmt.Sprintf("Setting up communicator for container %s...", ctid))

	state.Put(StateProxmoxComm, parentComm)

	comm := remote.NewPctExecCommunicator(ctid, parentComm)
	state.Put(StateCommunicator, comm)
	ui.Say(fmt.Sprintf("Communicator for container %s ready", ctid))
	return multistep.ActionContinue
}

func (s *SetupContainerComm) Cleanup(state multistep.StateBag) {}
