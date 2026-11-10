package steps

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/testutil"
)

func TestStepSetupContainerComm(t *testing.T) {
	cfg := &config.Config{}
	ui := &testutil.UI{}
	parentComm := &remote.SSHCommunicator{}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, parentComm)
	state.Put(StateCTID, "100")

	step := &SetupContainerComm{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	// Check that communicator was replaced with a remote.PctExecCommunicator
	comm := state.Get(StateCommunicator)
	if _, ok := comm.(*remote.PctExecCommunicator); !ok {
		t.Errorf("Expected communicator to be *remote.PctExecCommunicator, got %T", comm)
	}

	// Check that proxmox_comm was set
	proxmoxComm := state.Get(StateProxmoxComm)
	if proxmoxComm != parentComm {
		t.Errorf("Expected proxmox_comm to be parentComm")
	}
}

func TestStepSetupContainerComm_Cleanup(t *testing.T) {
	step := &SetupContainerComm{}
	state := new(multistep.BasicStateBag)

	// Should not panic
	step.Cleanup(state)
}
