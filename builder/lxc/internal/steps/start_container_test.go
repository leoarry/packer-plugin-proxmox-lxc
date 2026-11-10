package steps

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/testutil"
)

func TestStepStartContainer(t *testing.T) {
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Outputs: []string{""}, Errors: []error{nil}}

	state := new(multistep.BasicStateBag)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &StartContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
}

func TestStepStartContainer_Error(t *testing.T) {
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Errors: []error{&someError{msg: "pct start failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &StartContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
}
