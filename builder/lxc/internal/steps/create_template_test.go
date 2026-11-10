package steps

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/testutil"
)

func TestStepCreateTemplate_Success(t *testing.T) {
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Outputs: []string{"", ""}, Errors: []error{nil, nil}}

	state := new(multistep.BasicStateBag)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &CreateTemplate{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	templated, ok := state.Get(StateContainerTemplated).(bool)
	if !ok || !templated {
		t.Errorf("Expected container_templated true")
	}

	expectedCommands := []string{"pct stop 100", "pct template 100"}
	if len(comm.Calls) != len(expectedCommands) {
		t.Fatalf("Expected %d commands, got %d", len(expectedCommands), len(comm.Calls))
	}
	for i, expected := range expectedCommands {
		if comm.Calls[i] != expected {
			t.Errorf("Expected command %d to be %q, got %q", i, expected, comm.Calls[i])
		}
	}
}

func TestStepCreateTemplate_StopError(t *testing.T) {
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Errors: []error{&someError{msg: "pct stop failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &CreateTemplate{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if _, ok := state.GetOk(StateError); !ok {
		t.Errorf("Expected error in state")
	}
	if _, ok := state.GetOk(StateContainerTemplated); ok {
		t.Errorf("Expected container_templated not set")
	}
}

func TestStepCreateTemplate_TemplateError(t *testing.T) {
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{
		Outputs: []string{"", ""},
		Errors:  []error{nil, &someError{msg: "pct template failed"}},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &CreateTemplate{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if _, ok := state.GetOk(StateError); !ok {
		t.Errorf("Expected error in state")
	}
	if _, ok := state.GetOk(StateContainerTemplated); ok {
		t.Errorf("Expected container_templated not set")
	}
}

func TestStepCreateTemplate_Cleanup(t *testing.T) {
	step := &CreateTemplate{}
	state := new(multistep.BasicStateBag)

	// Should not panic
	step.Cleanup(state)
}
