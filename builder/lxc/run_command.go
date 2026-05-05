package lxc

import (
	"context"
	"io"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

// CommandRunner is an interface for running commands.
// It is implemented by sshCommunicator and can be mocked for testing.
type CommandRunner interface {
	RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error
}

// Helper to check if a step should halt.
func stepHalt(state multistep.StateBag) multistep.StepAction {
	if v, ok := state.GetOk("error"); ok && v != nil {
		return multistep.ActionHalt
	}
	return multistep.ActionContinue
}
