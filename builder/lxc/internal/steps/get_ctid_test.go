package steps

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/testutil"
)

func TestStepGetCTID_WithConfigCTID(t *testing.T) {
	cfg := &config.Config{
		CTID: "100",
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)

	step := &GetCTID{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	ctid, ok := state.Get(StateCTID).(string)
	if !ok || ctid != "100" {
		t.Errorf("Expected ctid '100', got %v", ctid)
	}
}

func TestStepGetCTID_WithoutCTID(t *testing.T) {
	cfg := &config.Config{}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Outputs: []string{"200\n"}, Errors: []error{nil}}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)

	step := &GetCTID{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	ctid, ok := state.Get(StateCTID).(string)
	if !ok || ctid != "200" {
		t.Errorf("Expected ctid '200', got %v", ctid)
	}
}

func TestStepGetCTID_WithoutCTID_Error(t *testing.T) {
	cfg := &config.Config{}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Errors: []error{&someError{msg: "command failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)

	step := &GetCTID{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}

	if _, ok := state.GetOk(StateError); !ok {
		t.Errorf("Expected error in state")
	}
}

func TestStepGetCTID_WithoutCTID_Empty(t *testing.T) {
	cfg := &config.Config{}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Outputs: []string{"\n"}, Errors: []error{nil}}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)

	step := &GetCTID{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
}

func TestStepGetCTID_Cleanup(t *testing.T) {
	step := &GetCTID{}
	state := new(multistep.BasicStateBag)

	// Should not panic
	step.Cleanup(state)
}
