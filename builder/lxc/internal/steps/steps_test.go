package steps

import (
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

// Test step interfaces are properly implemented
func TestSteps_ImplementStep(t *testing.T) {
	steps := []multistep.Step{
		&Connect{},
		&GetCTID{},
		&CreateContainer{},
		&MergeConfig{},
		&StartContainer{},
		&SetupContainerComm{},
		&BackupContainer{},
		&DestroyContainer{},
		&CreateTemplate{},
	}

	if len(steps) != 9 {
		t.Errorf("Expected 9 steps, got %d", len(steps))
	}
}

// Test_CleanupFunctions tests that all Cleanup functions don't panic
func Test_CleanupFunctions(t *testing.T) {
	// Test step Cleanup functions with various state bags

	steps := []struct {
		name string
		step multistep.Step
	}{
		{"Connect", &Connect{}},
		{"GetCTID", &GetCTID{}},
		{"CreateContainer", &CreateContainer{}},
		{"MergeConfig", &MergeConfig{}},
		{"StartContainer", &StartContainer{}},
		{"SetupContainerComm", &SetupContainerComm{}},
		{"BackupContainer", &BackupContainer{}},
		{"DestroyContainer", &DestroyContainer{}},
	}

	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			// Test with empty state
			state := new(multistep.BasicStateBag)
			s.step.Cleanup(state)

			// Test with SSH client in state (for Connect)
			if sc, ok := s.step.(*Connect); ok {
				_ = sc
				// We can't easily add a mock ssh.Client, so just test empty state
			}
		})
	}
}
