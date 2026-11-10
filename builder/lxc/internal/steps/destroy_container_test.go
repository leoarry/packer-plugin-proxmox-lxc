package steps

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/testutil"
)

func TestStepDestroyContainer(t *testing.T) {
	ui := &testutil.UI{}
	// Sequence: pct stop (error OK), pct destroy (error OK), rm -f
	comm := &testutil.MockRunner{
		Outputs: []string{"", "", ""},
		Errors:  []error{&someError{msg: "stop failed"}, &someError{msg: "destroy failed"}, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &DestroyContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
}

func TestStepDestroyContainer_Error(t *testing.T) {
	ui := &testutil.UI{}
	// Errors from stop/destroy are logged but don't halt
	comm := &testutil.MockRunner{Errors: []error{&someError{msg: "pct stop failed"}, &someError{msg: "pct destroy failed"}, nil}}

	state := new(multistep.BasicStateBag)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &DestroyContainer{}
	action := step.Run(context.Background(), state)

	// Errors from stop/destroy are logged but don't halt
	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue (errors non-fatal), got %v", action)
	}
}
