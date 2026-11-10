package steps

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/testutil"
)

func TestStepMergeConfig_NoConfig(t *testing.T) {
	cfg := &config.Config{
		LXCConfig: "",
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &MergeConfig{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
}

func TestStepMergeConfig_WithConfig(t *testing.T) {
	cfg := &config.Config{
		LXCConfig: "lxc.apparmor.profile: unconfined",
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Outputs: []string{""}, Errors: []error{nil}}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &MergeConfig{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
}

func TestStepMergeConfig_WithConfig_Error(t *testing.T) {
	cfg := &config.Config{
		LXCConfig: "lxc.apparmor.profile: unconfined",
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Errors: []error{&someError{msg: "echo failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &MergeConfig{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
}

func TestStepMergeConfig_Cleanup(t *testing.T) {
	step := &MergeConfig{}
	state := new(multistep.BasicStateBag)

	// Should not panic
	step.Cleanup(state)
}
