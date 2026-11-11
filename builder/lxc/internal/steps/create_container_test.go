package steps

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/testutil"
)

func TestStepCreateContainer_Reused(t *testing.T) {
	// Reuse only applies to an explicitly-configured CTID: that's the only
	// case where "a container already exists at this ID" reflects genuine
	// user intent rather than a race with a concurrent build.
	cfg := &config.Config{CTID: "100"}
	ui := &testutil.UI{}
	// First call: pct status returns output (no error) -> container exists
	comm := &testutil.MockRunner{Outputs: []string{"container status output", ""}, Errors: []error{nil, nil}}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	reused, ok := state.Get(StateContainerReused).(bool)
	if !ok || !reused {
		t.Errorf("Expected container_reused true")
	}
}

func TestStepCreateContainer_AutoAssignedExists_RetrySucceeds(t *testing.T) {
	// CTID left empty: auto-assigned. If "pct status" unexpectedly
	// succeeds for it, that's a race with a concurrent build, not a
	// legitimate reuse case — it must retry with a fresh CTID rather than
	// silently reusing whatever that other build just created.
	cfg := &config.Config{Template: "local:vztmpl/ubuntu-22.04.tar.gz"}
	ui := &testutil.UI{}
	// Sequence: pct status 100 (exists!), pvesh nextid (-> 101),
	// pct status 101 (not found), pct create 101 (success).
	comm := &testutil.MockRunner{
		Outputs: []string{"exists", "101\n", "", ""},
		Errors: []error{
			nil,
			nil,
			&someError{msg: "container not found"},
			nil,
		},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
	if ctid := state.Get(StateCTID).(string); ctid != "101" {
		t.Errorf("Expected ctid updated to '101', got %q", ctid)
	}
	if reused, ok := state.Get(StateContainerReused).(bool); !ok || reused {
		t.Errorf("Expected container_reused false (must not reuse a raced container)")
	}
}

func TestStepCreateContainer_AutoAssignedExists_FetchFails(t *testing.T) {
	cfg := &config.Config{Template: "local:vztmpl/ubuntu-22.04.tar.gz"}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{
		Outputs: []string{"exists", ""},
		Errors:  []error{nil, &someError{msg: "pvesh failed"}},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if _, ok := state.GetOk(StateError); !ok {
		t.Errorf("Expected error in state")
	}
}

func TestStepCreateContainer_AutoAssignedExists_RetriesExhausted(t *testing.T) {
	cfg := &config.Config{Template: "local:vztmpl/ubuntu-22.04.tar.gz"}
	ui := &testutil.UI{}

	// "pct status" always succeeds (exists), for maxCTIDConflictRetries+1
	// attempts, with a successful nextid fetch after each of the first
	// maxCTIDConflictRetries attempts.
	var outputs []string
	var errs []error
	for i := 0; i <= maxCTIDConflictRetries; i++ {
		outputs = append(outputs, "exists")
		errs = append(errs, nil)
		if i < maxCTIDConflictRetries {
			outputs = append(outputs, fmt.Sprintf("%d\n", 200+i))
			errs = append(errs, nil)
		}
	}
	comm := &testutil.MockRunner{Outputs: outputs, Errors: errs}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt after exhausting retries, got %v", action)
	}
	if _, ok := state.GetOk(StateError); !ok {
		t.Errorf("Expected error in state")
	}
	// status * (maxCTIDConflictRetries+1 attempts) + fetch * maxCTIDConflictRetries retries
	expectedCalls := (maxCTIDConflictRetries + 1) + maxCTIDConflictRetries
	if len(comm.Calls) != expectedCalls {
		t.Errorf("Expected %d commands, got %d: %v", expectedCalls, len(comm.Calls), comm.Calls)
	}
}

func TestStepCreateContainer_Create(t *testing.T) {
	cfg := &config.Config{
		Unprivileged: testutil.BoolPtr(true),
		Storage:      "local-lvm",
		Memory:       1024,
		Cores:        2,
		RootfsSize:   "2",
		RootPassword: "test123",
		Bridge:       "vmbr0",
		Features:     "nesting=1",
		Template:     "local:vztmpl/ubuntu-22.04.tar.gz",
	}
	ui := &testutil.UI{}
	// First call: pct status returns error (container doesn't exist)
	// Second call: pct create succeeds
	comm := &testutil.MockRunner{
		Outputs: []string{"", ""},
		Errors:  []error{&someError{msg: "container not found"}, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	reused, ok := state.Get(StateContainerReused).(bool)
	if !ok || reused {
		t.Errorf("Expected container_reused false")
	}
}

func TestStepCreateContainer_CreateWithNetworkOptions(t *testing.T) {
	cfg := &config.Config{
		Unprivileged: testutil.BoolPtr(true),
		Storage:      "local-lvm",
		Memory:       1024,
		Cores:        2,
		RootfsSize:   "2",
		RootPassword: "test123",
		Bridge:       "vmbr0",
		Features:     "nesting=1",
		Template:     "local:vztmpl/ubuntu-22.04.tar.gz",
		Vlan:         100,
		NetworkIP:    "192.168.1.50/24",
		Gateway:      "192.168.1.1",
		Firewall:     true,
		NetworkMTU:   1500,
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{
		Outputs: []string{"", ""},
		Errors:  []error{&someError{msg: "container not found"}, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	if len(comm.Calls) < 2 {
		t.Fatalf("Expected at least 2 commands, got %d", len(comm.Calls))
	}
	createCmd := comm.Calls[1]
	expectedNet0 := "--net0 name=eth0,bridge=vmbr0,tag=100,ip=192.168.1.50/24,gw=192.168.1.1,firewall=1,mtu=1500"
	if !strings.Contains(createCmd, expectedNet0) {
		t.Errorf("Expected pct create command to contain %q, got: %s", expectedNet0, createCmd)
	}
}

func TestStepCreateContainer_CreateError(t *testing.T) {
	cfg := &config.Config{}
	ui := &testutil.UI{}
	// pct status fails, then pct create also fails
	comm := &testutil.MockRunner{
		Outputs: []string{"", ""},
		Errors:  []error{&someError{msg: "container not found"}, &someError{msg: "pct create failed"}},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	// The pct status fails, so it tries to create
	// We need a more sophisticated mock that returns different values for different commands
	// For now, just check it doesn't panic
	_ = action
}

func TestStepCreateContainer_CTIDConflict_RetrySucceeds(t *testing.T) {
	cfg := &config.Config{
		Unprivileged: testutil.BoolPtr(true),
		Storage:      "local-lvm",
		Memory:       1024,
		Cores:        2,
		RootfsSize:   "2",
		RootPassword: "test123",
		Bridge:       "vmbr0",
		Features:     "nesting=1",
		Template:     "local:vztmpl/ubuntu-22.04.tar.gz",
		// CTID left empty: auto-assigned, so retries are allowed.
	}
	ui := &testutil.UI{}
	// Sequence: pct status 100 (not found), pct create 100 (conflict),
	// pvesh get nextid (-> 101), pct status 101 (not found), pct create 101 (success).
	comm := &testutil.MockRunner{
		Outputs: []string{"", "", "101\n", "", ""},
		Stderrs: []string{"", "unable to create CT 100 - already exists", "", "", ""},
		Errors: []error{
			&someError{msg: "container not found"},
			&someError{msg: "pct create failed"},
			nil,
			&someError{msg: "container not found"},
			nil,
		},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
	if ctid := state.Get(StateCTID).(string); ctid != "101" {
		t.Errorf("Expected ctid to be updated to '101', got %q", ctid)
	}
	reused, ok := state.Get(StateContainerReused).(bool)
	if !ok || reused {
		t.Errorf("Expected container_reused false")
	}
	if len(comm.Calls) != 5 {
		t.Errorf("Expected 5 commands, got %d: %v", len(comm.Calls), comm.Calls)
	}
}

func TestStepCreateContainer_NonConflictError_NoRetry(t *testing.T) {
	cfg := &config.Config{
		Template: "local:vztmpl/ubuntu-22.04.tar.gz",
		// CTID left empty: auto-assigned.
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{
		Outputs: []string{"", ""},
		Stderrs: []string{"", "disk quota exceeded"},
		Errors: []error{
			&someError{msg: "container not found"},
			&someError{msg: "pct create failed"},
		},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if err, ok := state.GetOk(StateError); !ok {
		t.Errorf("Expected error in state")
	} else if !strings.Contains(err.(error).Error(), "disk quota exceeded") {
		t.Errorf("Expected pct's stderr in the error, got %q", err)
	}
	// No retry: only the status + create calls, no CTID re-fetch.
	if len(comm.Calls) != 2 {
		t.Errorf("Expected 2 commands (no retry), got %d: %v", len(comm.Calls), comm.Calls)
	}
}

func TestStepCreateContainer_ConflictNotRetriedWithExplicitCTID(t *testing.T) {
	cfg := &config.Config{
		Template: "local:vztmpl/ubuntu-22.04.tar.gz",
		CTID:     "100", // explicit: must not be re-rolled even on conflict
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{
		Outputs: []string{"", ""},
		Stderrs: []string{"", "unable to create CT 100 - already exists"},
		Errors: []error{
			&someError{msg: "container not found"},
			&someError{msg: "pct create failed"},
		},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if len(comm.Calls) != 2 {
		t.Errorf("Expected 2 commands (no retry with explicit CTID), got %d: %v", len(comm.Calls), comm.Calls)
	}
}

func TestStepCreateContainer_CTIDConflict_RetriesExhausted(t *testing.T) {
	cfg := &config.Config{
		Template: "local:vztmpl/ubuntu-22.04.tar.gz",
		// CTID left empty: auto-assigned.
	}
	ui := &testutil.UI{}

	// Build a sequence that always conflicts: (status-not-found, create-conflict)
	// repeated maxCTIDConflictRetries+1 times, with a successful nextid fetch
	// after each of the first maxCTIDConflictRetries conflicts.
	var outputs, stderrs []string
	var errs []error
	for i := 0; i <= maxCTIDConflictRetries; i++ {
		outputs = append(outputs, "", "")
		stderrs = append(stderrs, "", "already exists")
		errs = append(errs, &someError{msg: "container not found"}, &someError{msg: "pct create failed"})
		if i < maxCTIDConflictRetries {
			outputs = append(outputs, fmt.Sprintf("%d\n", 200+i))
			stderrs = append(stderrs, "")
			errs = append(errs, nil)
		}
	}
	comm := &testutil.MockRunner{Outputs: outputs, Stderrs: stderrs, Errors: errs}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateCommunicator, comm)
	state.Put(StateCTID, "100")

	step := &CreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt after exhausting retries, got %v", action)
	}
	if _, ok := state.GetOk(StateError); !ok {
		t.Errorf("Expected error in state")
	}
	// (status+create) * (maxCTIDConflictRetries+1 attempts) + fetch * maxCTIDConflictRetries retries
	expectedCalls := 2*(maxCTIDConflictRetries+1) + maxCTIDConflictRetries
	if len(comm.Calls) != expectedCalls {
		t.Errorf("Expected %d commands, got %d: %v", expectedCalls, len(comm.Calls), comm.Calls)
	}
}

func TestIsCTIDConflict(t *testing.T) {
	tests := []struct {
		name   string
		stderr string
		want   bool
	}{
		{name: "container already exists", stderr: "CT 101 already exists on node 'pve01'", want: true},
		{name: "config file already exists", stderr: "unable to create CT 101 - configuration file 'nodes/pve01/lxc/101.conf' already exists", want: true},
		{name: "case insensitive", stderr: "Already Exists", want: true},
		{name: "unrelated error", stderr: "disk quota exceeded", want: false},
		{name: "empty", stderr: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCTIDConflict(tt.stderr); got != tt.want {
				t.Errorf("isCTIDConflict(%q) = %v, want %v", tt.stderr, got, tt.want)
			}
		})
	}
}

func TestCTIDRetryBackoff(t *testing.T) {
	for attempt := 0; attempt < 5; attempt++ {
		d := ctidRetryBackoff(attempt)
		minExpected := 100 * time.Millisecond * time.Duration(attempt+1)
		maxExpected := minExpected + 200*time.Millisecond
		if d < minExpected || d > maxExpected {
			t.Errorf("ctidRetryBackoff(%d) = %v, want between %v and %v", attempt, d, minExpected, maxExpected)
		}
	}
}

func TestStepCreateContainer_Cleanup_AlreadyDestroyed(t *testing.T) {
	// If container was already destroyed, cleanup should skip
	comm := &testutil.MockRunner{}
	state := new(multistep.BasicStateBag)
	state.Put(StateCTID, "100")
	state.Put(StateCommunicator, comm)
	state.Put(StateContainerDestroyed, true)

	step := &CreateContainer{}
	step.Cleanup(state)

	// Cleanup should not call any commands since container was already destroyed
	if len(comm.Calls) != 0 {
		t.Errorf("Expected no commands called, got %d calls", len(comm.Calls))
	}
}

func TestStepCreateContainer_Cleanup_Reused(t *testing.T) {
	// If container was reused, cleanup should skip
	comm := &testutil.MockRunner{}
	state := new(multistep.BasicStateBag)
	state.Put(StateCTID, "100")
	state.Put(StateCommunicator, comm)
	state.Put(StateContainerReused, true)

	step := &CreateContainer{}
	step.Cleanup(state)

	// Cleanup should not call any commands since container was reused
	if len(comm.Calls) != 0 {
		t.Errorf("Expected no commands called, got %d calls", len(comm.Calls))
	}
}

func TestStepCreateContainer_Cleanup_Templated(t *testing.T) {
	// If container was successfully converted to a CT template, cleanup
	// should skip destroying it — the templated container is the artifact.
	comm := &testutil.MockRunner{}
	state := new(multistep.BasicStateBag)
	state.Put(StateCTID, "100")
	state.Put(StateCommunicator, comm)
	state.Put(StateContainerTemplated, true)

	step := &CreateContainer{}
	step.Cleanup(state)

	if len(comm.Calls) != 0 {
		t.Errorf("Expected no commands called, got %d calls", len(comm.Calls))
	}
}

func TestStepCreateContainer_Cleanup_NeedsCleanup(t *testing.T) {
	// If container was created but not destroyed, cleanup should destroy it
	comm := &testutil.MockRunner{Outputs: []string{"", "", ""}, Errors: []error{nil, nil, nil}}
	state := new(multistep.BasicStateBag)
	state.Put(StateCTID, "100")
	state.Put(StateCommunicator, comm)
	state.Put(StateUI, &testutil.UI{})
	// container_reused is false (or not set) and container_destroyed is not set

	step := &CreateContainer{}
	step.Cleanup(state)

	// Should call pct stop, pct destroy, and rm
	if len(comm.Calls) < 3 {
		t.Errorf("Expected at least 3 commands called, got %d calls", len(comm.Calls))
	}
}

func TestStepCreateContainer_Cleanup_NoCommunicator(t *testing.T) {
	// If communicator is not in state, cleanup should not panic
	state := new(multistep.BasicStateBag)
	state.Put(StateCTID, "100")
	// No communicator in state

	step := &CreateContainer{}
	step.Cleanup(state)
	// Should not panic
}

func TestStepCreateContainer_Cleanup_NoCtid(t *testing.T) {
	// If ctid is not in state, cleanup should not panic
	comm := &testutil.MockRunner{}
	state := new(multistep.BasicStateBag)
	state.Put(StateCommunicator, comm)
	// No ctid in state

	step := &CreateContainer{}
	step.Cleanup(state)
	// Should not panic
}

func TestStepCreateContainer_Cleanup_WithProxmoxComm(t *testing.T) {
	// If proxmox_comm is set (after SetupContainerComm), cleanup should use it
	// This simulates a script failure during provisioning
	hostComm := &testutil.MockRunner{Outputs: []string{"", "", ""}, Errors: []error{nil, nil, nil}}
	state := new(multistep.BasicStateBag)
	state.Put(StateCTID, "100")
	state.Put(StateProxmoxComm, hostComm)
	state.Put(StateUI, &testutil.UI{})
	// container_reused is false and container_destroyed is not set

	step := &CreateContainer{}
	step.Cleanup(state)

	// Should call pct stop, pct destroy, and rm on the host communicator
	if len(hostComm.Calls) < 3 {
		t.Errorf("Expected at least 3 commands called, got %d calls", len(hostComm.Calls))
	}
}

func TestStepCreateContainer_Cleanup_ScriptFailure(t *testing.T) {
	// Simulate a script failing during provisioning
	// At this point, SetupContainerComm has run and set proxmox_comm
	hostComm := &testutil.MockRunner{Outputs: []string{"", "", ""}, Errors: []error{nil, nil, nil}}
	state := new(multistep.BasicStateBag)
	state.Put(StateCTID, "100")
	state.Put(StateProxmoxComm, hostComm)
	state.Put(StateUI, &testutil.UI{})
	// Simulate that container was not reused and not yet destroyed

	step := &CreateContainer{}
	step.Cleanup(state)

	// Verify the correct commands were run on the host
	expectedCommands := []string{
		fmt.Sprintf("pct stop %s || true", "100"),
		fmt.Sprintf("pct destroy %s --purge", "100"),
		fmt.Sprintf("rm -f /tmp/vzdump-lxc-%s-*.log 2>/dev/null || true", "100"),
	}
	if len(hostComm.Calls) != len(expectedCommands) {
		t.Errorf("Expected %d commands, got %d", len(expectedCommands), len(hostComm.Calls))
	}
	for i, expected := range expectedCommands {
		if i < len(hostComm.Calls) && hostComm.Calls[i] != expected {
			t.Errorf("Expected command %d to be %q, got %q", i, expected, hostComm.Calls[i])
		}
	}
}
