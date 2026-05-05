package lxc

import (
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

func TestStepHalt_NoError(t *testing.T) {
	state := new(multistep.BasicStateBag)

	action := stepHalt(state)
	if action != multistep.ActionContinue {
		t.Errorf("stepHalt() = %v, want %v", action, multistep.ActionContinue)
	}
}

func TestStepHalt_WithNilError(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("error", nil)

	action := stepHalt(state)
	if action != multistep.ActionContinue {
		t.Errorf("stepHalt() with nil error = %v, want %v", action, multistep.ActionContinue)
	}
}

func TestStepHalt_WithError(t *testing.T) {
	tests := []struct {
		name     string
		errorVal interface{}
		wantHalt multistep.StepAction
	}{
		{
			name:     "string error",
			errorVal: "something went wrong",
			wantHalt: multistep.ActionHalt,
		},
		{
			name:     "nil error",
			errorVal: nil,
			wantHalt: multistep.ActionContinue,
		},
		{
			name:     "no error key",
			errorVal: nil,
			wantHalt: multistep.ActionContinue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := new(multistep.BasicStateBag)
			if tt.errorVal != nil {
				state.Put("error", tt.errorVal)
			}

			action := stepHalt(state)
			if action != tt.wantHalt {
				t.Errorf("stepHalt() = %v, want %v", action, tt.wantHalt)
			}
		})
	}
}

func TestStepHalt_WithErrorInterface(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("error", &testError{msg: "test error"})

	action := stepHalt(state)
	if action != multistep.ActionHalt {
		t.Errorf("stepHalt() with error interface = %v, want %v", action, multistep.ActionHalt)
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
