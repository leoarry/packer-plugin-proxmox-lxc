package lxc

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// testUi is a simple mock UI for testing
type testUi struct{}

func (u *testUi) Say(message string)                                      {}
func (u *testUi) Sayf(format string, args ...interface{})                 {}
func (u *testUi) Message(message string)                                  {}
func (u *testUi) Error(message string)                                    {}
func (u *testUi) Errorf(format string, args ...interface{})               {}
func (u *testUi) Ask(query string) (string, error)                        { return "", nil }
func (u *testUi) Askf(format string, args ...interface{}) (string, error) { return "", nil }
func (u *testUi) Machine(name string, args ...string)                     {}
func (u *testUi) TrackProgress(src string, currentSize, totalSize int64, stream io.ReadCloser) io.ReadCloser {
	return stream
}

// Ensure testUi implements packersdk.Ui
var _ packersdk.Ui = &testUi{}

func TestStepGetCTID_WithConfigCTID(t *testing.T) {
	config := &Config{
		CTID: "100",
	}
	ui := &testUi{}
	comm := &mockCommandRunner{}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)

	step := &stepGetCTID{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	ctid, ok := state.Get("ctid").(string)
	if !ok || ctid != "100" {
		t.Errorf("Expected ctid '100', got %v", ctid)
	}
}

func TestStepGetCTID_WithoutCTID(t *testing.T) {
	config := &Config{}
	ui := &testUi{}
	comm := &mockCommandRunner{outputs: []string{"200\n"}, errors: []error{nil}}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)

	step := &stepGetCTID{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	ctid, ok := state.Get("ctid").(string)
	if !ok || ctid != "200" {
		t.Errorf("Expected ctid '200', got %v", ctid)
	}
}

func TestStepGetCTID_WithoutCTID_Error(t *testing.T) {
	config := &Config{}
	ui := &testUi{}
	comm := &mockCommandRunner{errors: []error{&someError{msg: "command failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)

	step := &stepGetCTID{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}

	if _, ok := state.GetOk("error"); !ok {
		t.Errorf("Expected error in state")
	}
}

func TestStepGetCTID_WithoutCTID_Empty(t *testing.T) {
	config := &Config{}
	ui := &testUi{}
	comm := &mockCommandRunner{outputs: []string{"\n"}, errors: []error{nil}}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)

	step := &stepGetCTID{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
}

func TestStepGetCTID_Cleanup(t *testing.T) {
	step := &stepGetCTID{}
	state := new(multistep.BasicStateBag)

	// Should not panic
	step.Cleanup(state)
}

func TestStepMergeConfig_NoConfig(t *testing.T) {
	config := &Config{
		LXCConfig: "",
	}
	ui := &testUi{}
	comm := &mockCommandRunner{}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepMergeConfig{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
}

func TestStepMergeConfig_WithConfig(t *testing.T) {
	config := &Config{
		LXCConfig: "lxc.apparmor.profile: unconfined",
	}
	ui := &testUi{}
	comm := &mockCommandRunner{outputs: []string{""}, errors: []error{nil}}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepMergeConfig{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
}

func TestStepMergeConfig_WithConfig_Error(t *testing.T) {
	config := &Config{
		LXCConfig: "lxc.apparmor.profile: unconfined",
	}
	ui := &testUi{}
	comm := &mockCommandRunner{errors: []error{&someError{msg: "echo failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepMergeConfig{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
}

func TestStepMergeConfig_Cleanup(t *testing.T) {
	step := &stepMergeConfig{}
	state := new(multistep.BasicStateBag)

	// Should not panic
	step.Cleanup(state)
}

func TestStepStartContainer(t *testing.T) {
	ui := &testUi{}
	comm := &mockCommandRunner{outputs: []string{""}, errors: []error{nil}}

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepStartContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
}

func TestStepStartContainer_Error(t *testing.T) {
	ui := &testUi{}
	comm := &mockCommandRunner{errors: []error{&someError{msg: "pct start failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepStartContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
}

func TestStepCreateContainer_Reused(t *testing.T) {
	// Reuse only applies to an explicitly-configured CTID: that's the only
	// case where "a container already exists at this ID" reflects genuine
	// user intent rather than a race with a concurrent build.
	config := &Config{CTID: "100"}
	ui := &testUi{}
	// First call: pct status returns output (no error) -> container exists
	comm := &mockCommandRunner{outputs: []string{"container status output", ""}, errors: []error{nil, nil}}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	reused, ok := state.Get("container_reused").(bool)
	if !ok || !reused {
		t.Errorf("Expected container_reused true")
	}
}

func TestStepCreateContainer_AutoAssignedExists_RetrySucceeds(t *testing.T) {
	// CTID left empty: auto-assigned. If "pct status" unexpectedly
	// succeeds for it, that's a race with a concurrent build, not a
	// legitimate reuse case — it must retry with a fresh CTID rather than
	// silently reusing whatever that other build just created.
	config := &Config{Template: "local:vztmpl/ubuntu-22.04.tar.gz"}
	ui := &testUi{}
	// Sequence: pct status 100 (exists!), pvesh nextid (-> 101),
	// pct status 101 (not found), pct create 101 (success).
	comm := &mockCommandRunner{
		outputs: []string{"exists", "101\n", "", ""},
		errors: []error{
			nil,
			nil,
			&someError{msg: "container not found"},
			nil,
		},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
	if ctid := state.Get("ctid").(string); ctid != "101" {
		t.Errorf("Expected ctid updated to '101', got %q", ctid)
	}
	if reused, ok := state.Get("container_reused").(bool); !ok || reused {
		t.Errorf("Expected container_reused false (must not reuse a raced container)")
	}
}

func TestStepCreateContainer_AutoAssignedExists_FetchFails(t *testing.T) {
	config := &Config{Template: "local:vztmpl/ubuntu-22.04.tar.gz"}
	ui := &testUi{}
	comm := &mockCommandRunner{
		outputs: []string{"exists", ""},
		errors:  []error{nil, &someError{msg: "pvesh failed"}},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if _, ok := state.GetOk("error"); !ok {
		t.Errorf("Expected error in state")
	}
}

func TestStepCreateContainer_AutoAssignedExists_RetriesExhausted(t *testing.T) {
	config := &Config{Template: "local:vztmpl/ubuntu-22.04.tar.gz"}
	ui := &testUi{}

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
	comm := &mockCommandRunner{outputs: outputs, errors: errs}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt after exhausting retries, got %v", action)
	}
	if _, ok := state.GetOk("error"); !ok {
		t.Errorf("Expected error in state")
	}
	// status * (maxCTIDConflictRetries+1 attempts) + fetch * maxCTIDConflictRetries retries
	expectedCalls := (maxCTIDConflictRetries + 1) + maxCTIDConflictRetries
	if len(comm.calls) != expectedCalls {
		t.Errorf("Expected %d commands, got %d: %v", expectedCalls, len(comm.calls), comm.calls)
	}
}

func TestStepCreateContainer_Create(t *testing.T) {
	config := &Config{
		Unprivileged: boolPtr(true),
		Storage:      "local-lvm",
		Memory:       1024,
		Cores:        2,
		RootfsSize:   "2",
		RootPassword: "test123",
		Bridge:       "vmbr0",
		Features:     "nesting=1",
		Template:     "local:vztmpl/ubuntu-22.04.tar.gz",
	}
	ui := &testUi{}
	// First call: pct status returns error (container doesn't exist)
	// Second call: pct create succeeds
	comm := &mockCommandRunner{
		outputs: []string{"", ""},
		errors:  []error{&someError{msg: "container not found"}, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	reused, ok := state.Get("container_reused").(bool)
	if !ok || reused {
		t.Errorf("Expected container_reused false")
	}
}

func TestStepCreateContainer_CreateWithNetworkOptions(t *testing.T) {
	config := &Config{
		Unprivileged: boolPtr(true),
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
	ui := &testUi{}
	comm := &mockCommandRunner{
		outputs: []string{"", ""},
		errors:  []error{&someError{msg: "container not found"}, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	if len(comm.calls) < 2 {
		t.Fatalf("Expected at least 2 commands, got %d", len(comm.calls))
	}
	createCmd := comm.calls[1]
	expectedNet0 := "--net0 name=eth0,bridge=vmbr0,tag=100,ip=192.168.1.50/24,gw=192.168.1.1,firewall=1,mtu=1500"
	if !strings.Contains(createCmd, expectedNet0) {
		t.Errorf("Expected pct create command to contain %q, got: %s", expectedNet0, createCmd)
	}
}

func TestStepCreateContainer_CreateError(t *testing.T) {
	config := &Config{}
	ui := &testUi{}
	// pct status fails, then pct create also fails
	comm := &mockCommandRunner{
		outputs: []string{"", ""},
		errors:  []error{&someError{msg: "container not found"}, &someError{msg: "pct create failed"}},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	// The pct status fails, so it tries to create
	// We need a more sophisticated mock that returns different values for different commands
	// For now, just check it doesn't panic
	_ = action
}

func TestStepCreateContainer_CTIDConflict_RetrySucceeds(t *testing.T) {
	config := &Config{
		Unprivileged: boolPtr(true),
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
	ui := &testUi{}
	// Sequence: pct status 100 (not found), pct create 100 (conflict),
	// pvesh get nextid (-> 101), pct status 101 (not found), pct create 101 (success).
	comm := &mockCommandRunner{
		outputs: []string{"", "", "101\n", "", ""},
		stderrs: []string{"", "unable to create CT 100 - already exists", "", "", ""},
		errors: []error{
			&someError{msg: "container not found"},
			&someError{msg: "pct create failed"},
			nil,
			&someError{msg: "container not found"},
			nil,
		},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
	if ctid := state.Get("ctid").(string); ctid != "101" {
		t.Errorf("Expected ctid to be updated to '101', got %q", ctid)
	}
	reused, ok := state.Get("container_reused").(bool)
	if !ok || reused {
		t.Errorf("Expected container_reused false")
	}
	if len(comm.calls) != 5 {
		t.Errorf("Expected 5 commands, got %d: %v", len(comm.calls), comm.calls)
	}
}

func TestStepCreateContainer_NonConflictError_NoRetry(t *testing.T) {
	config := &Config{
		Template: "local:vztmpl/ubuntu-22.04.tar.gz",
		// CTID left empty: auto-assigned.
	}
	ui := &testUi{}
	comm := &mockCommandRunner{
		outputs: []string{"", ""},
		stderrs: []string{"", "disk quota exceeded"},
		errors: []error{
			&someError{msg: "container not found"},
			&someError{msg: "pct create failed"},
		},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if _, ok := state.GetOk("error"); !ok {
		t.Errorf("Expected error in state")
	}
	// No retry: only the status + create calls, no CTID re-fetch.
	if len(comm.calls) != 2 {
		t.Errorf("Expected 2 commands (no retry), got %d: %v", len(comm.calls), comm.calls)
	}
}

func TestStepCreateContainer_ConflictNotRetriedWithExplicitCTID(t *testing.T) {
	config := &Config{
		Template: "local:vztmpl/ubuntu-22.04.tar.gz",
		CTID:     "100", // explicit: must not be re-rolled even on conflict
	}
	ui := &testUi{}
	comm := &mockCommandRunner{
		outputs: []string{"", ""},
		stderrs: []string{"", "unable to create CT 100 - already exists"},
		errors: []error{
			&someError{msg: "container not found"},
			&someError{msg: "pct create failed"},
		},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if len(comm.calls) != 2 {
		t.Errorf("Expected 2 commands (no retry with explicit CTID), got %d: %v", len(comm.calls), comm.calls)
	}
}

func TestStepCreateContainer_CTIDConflict_RetriesExhausted(t *testing.T) {
	config := &Config{
		Template: "local:vztmpl/ubuntu-22.04.tar.gz",
		// CTID left empty: auto-assigned.
	}
	ui := &testUi{}

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
	comm := &mockCommandRunner{outputs: outputs, stderrs: stderrs, errors: errs}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", comm)
	state.Put("ctid", "100")

	step := &stepCreateContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt after exhausting retries, got %v", action)
	}
	if _, ok := state.GetOk("error"); !ok {
		t.Errorf("Expected error in state")
	}
	// (status+create) * (maxCTIDConflictRetries+1 attempts) + fetch * maxCTIDConflictRetries retries
	expectedCalls := 2*(maxCTIDConflictRetries+1) + maxCTIDConflictRetries
	if len(comm.calls) != expectedCalls {
		t.Errorf("Expected %d commands, got %d: %v", expectedCalls, len(comm.calls), comm.calls)
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

func TestStepBackupContainer(t *testing.T) {
	config := &Config{
		BackupDir: "/var/lib/vz/template/cache",
	}
	ui := &testUi{}
	// Sequence: vzdump success, ls success, mkdir success, mv success
	comm := &mockCommandRunner{
		outputs: []string{"", "/tmp/vzdump-lxc-100-2026_0503.tar.gz", "", ""},
		errors:  []error{nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepBackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	if _, ok := state.GetOk("backup_path"); !ok {
		t.Errorf("Expected backup_path in state")
	}
}

// Each algorithm must reach vzdump AND drive the dump-file lookup and the
// artifact path, or the backup would succeed on the host and then be reported
// as missing because the step globbed for the wrong extension.
func TestStepBackupContainer_Compression(t *testing.T) {
	tests := []struct {
		name        string
		compression string
		wantVzdump  string
		wantLs      string
		wantPath    string
	}{
		{
			name:        "gzip",
			compression: "gzip",
			wantVzdump:  "vzdump 100 --compress gzip --dumpdir /tmp",
			wantLs:      "ls /tmp/vzdump-lxc-100-*.tar.gz 2>/dev/null | head -1",
			wantPath:    "/var/lib/vz/template/cache/lxc-template-100.tar.gz",
		},
		{
			name:        "zstd",
			compression: "zstd",
			wantVzdump:  "vzdump 100 --compress zstd --dumpdir /tmp",
			wantLs:      "ls /tmp/vzdump-lxc-100-*.tar.zst 2>/dev/null | head -1",
			wantPath:    "/var/lib/vz/template/cache/lxc-template-100.tar.zst",
		},
		{
			name:        "lzo",
			compression: "lzo",
			wantVzdump:  "vzdump 100 --compress lzo --dumpdir /tmp",
			wantLs:      "ls /tmp/vzdump-lxc-100-*.tar.lzo 2>/dev/null | head -1",
			wantPath:    "/var/lib/vz/template/cache/lxc-template-100.tar.lzo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				BackupDir:         "/var/lib/vz/template/cache",
				BackupCompression: tt.compression,
			}
			ui := &testUi{}
			// Sequence: vzdump, ls, mkdir, mv
			comm := &mockCommandRunner{
				outputs: []string{"", "/tmp/vzdump-lxc-100-2026_0503.dump", "", ""},
				errors:  []error{nil, nil, nil, nil},
			}

			state := new(multistep.BasicStateBag)
			state.Put("config", config)
			state.Put("ui", ui)
			state.Put("proxmox_comm", comm)
			state.Put("ctid", "100")

			step := &stepBackupContainer{}
			if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
				t.Fatalf("Expected ActionContinue, got %v", action)
			}

			if comm.calls[0] != tt.wantVzdump {
				t.Errorf("Expected vzdump command %q, got %q", tt.wantVzdump, comm.calls[0])
			}
			if comm.calls[1] != tt.wantLs {
				t.Errorf("Expected lookup command %q, got %q", tt.wantLs, comm.calls[1])
			}
			got, _ := state.Get("backup_path").(string)
			if got != tt.wantPath {
				t.Errorf("Expected backup_path %q, got %q", tt.wantPath, got)
			}
		})
	}
}

// pigz is a parallel gzip; vzdump ignores --pigz for other algorithms, so the
// step must not probe for it or pass it when compression is not gzip.
func TestStepBackupContainer_NonGzipIgnoresPigz(t *testing.T) {
	config := &Config{
		BackupDir:         "/var/lib/vz/template/cache",
		BackupCompression: "zstd",
		BackupPigz:        4,
	}
	ui := &testUi{}
	// Sequence: vzdump, ls, mkdir, mv -- deliberately NO `command -v pigz`.
	comm := &mockCommandRunner{
		outputs: []string{"", "/tmp/vzdump-lxc-100-2026_0503.tar.zst", "", ""},
		errors:  []error{nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepBackupContainer{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("Expected ActionContinue, got %v", action)
	}

	for _, call := range comm.calls {
		if strings.Contains(call, "pigz") {
			t.Errorf("Expected no pigz handling for zstd, got command %q", call)
		}
	}
	expected := "vzdump 100 --compress zstd --dumpdir /tmp"
	if comm.calls[0] != expected {
		t.Errorf("Expected vzdump command %q, got %q", expected, comm.calls[0])
	}
}

func TestStepBackupContainer_WithPigz(t *testing.T) {
	config := &Config{
		BackupDir:         "/var/lib/vz/template/cache",
		BackupCompression: "gzip",
		BackupPigz:        4,
	}
	ui := &testUi{}
	comm := &mockCommandRunner{
		// Sequence: command -v pigz (found), vzdump, ls, mkdir, mv
		outputs: []string{"", "", "/tmp/vzdump-lxc-100-2026_0503.tar.gz", "", ""},
		errors:  []error{nil, nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepBackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	if len(comm.calls) < 2 {
		t.Fatalf("Expected at least 2 commands, got %d", len(comm.calls))
	}
	if comm.calls[0] != "command -v pigz" {
		t.Errorf("Expected first command to check for pigz, got %q", comm.calls[0])
	}
	expectedVzdumpCmd := "vzdump 100 --compress gzip --dumpdir /tmp --pigz 4"
	if comm.calls[1] != expectedVzdumpCmd {
		t.Errorf("Expected vzdump command %q, got %q", expectedVzdumpCmd, comm.calls[1])
	}
}

func TestStepBackupContainer_PigzMissing_FallsBackToGzip(t *testing.T) {
	config := &Config{
		BackupDir:         "/var/lib/vz/template/cache",
		BackupCompression: "gzip",
		BackupPigz:        4,
	}
	ui := &testUi{}
	comm := &mockCommandRunner{
		// Sequence: command -v pigz (NOT found), vzdump (plain gzip), ls, mkdir, mv
		outputs: []string{"", "", "/tmp/vzdump-lxc-100-2026_0503.tar.gz", "", ""},
		errors:  []error{&someError{msg: "pigz: not found"}, nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepBackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	if len(comm.calls) < 2 {
		t.Fatalf("Expected at least 2 commands, got %d", len(comm.calls))
	}
	expectedVzdumpCmd := "vzdump 100 --compress gzip --dumpdir /tmp"
	if comm.calls[1] != expectedVzdumpCmd {
		t.Errorf("Expected fallback to plain gzip %q, got %q", expectedVzdumpCmd, comm.calls[1])
	}
}

func TestStepBackupContainer_PigzDisabled(t *testing.T) {
	config := &Config{
		BackupDir:         "/var/lib/vz/template/cache",
		BackupCompression: "gzip",
		BackupPigz:        -1, // explicitly disabled (resolved value after Config.Prepare)
	}
	ui := &testUi{}
	comm := &mockCommandRunner{
		outputs: []string{"", "/tmp/vzdump-lxc-100-2026_0503.tar.gz", "", ""},
		errors:  []error{nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepBackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	expectedVzdumpCmd := "vzdump 100 --compress gzip --dumpdir /tmp"
	if len(comm.calls) == 0 || comm.calls[0] != expectedVzdumpCmd {
		t.Errorf("Expected vzdump command %q, got %q", expectedVzdumpCmd, comm.calls[0])
	}
}

func TestStepBackupContainer_Error(t *testing.T) {
	config := &Config{}
	ui := &testUi{}
	comm := &mockCommandRunner{errors: []error{&someError{msg: "vzdump failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepBackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
}

func TestStepDestroyContainer(t *testing.T) {
	ui := &testUi{}
	// Sequence: pct stop (error OK), pct destroy (error OK), rm -f
	comm := &mockCommandRunner{
		outputs: []string{"", "", ""},
		errors:  []error{&someError{msg: "stop failed"}, &someError{msg: "destroy failed"}, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepDestroyContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}
}

func TestStepDestroyContainer_Error(t *testing.T) {
	ui := &testUi{}
	// Errors from stop/destroy are logged but don't halt
	comm := &mockCommandRunner{errors: []error{&someError{msg: "pct stop failed"}, &someError{msg: "pct destroy failed"}, nil}}

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepDestroyContainer{}
	action := step.Run(context.Background(), state)

	// Errors from stop/destroy are logged but don't halt
	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue (errors non-fatal), got %v", action)
	}
}

func TestStepSetupContainerComm(t *testing.T) {
	config := &Config{}
	ui := &testUi{}
	parentComm := &sshCommunicator{}

	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", ui)
	state.Put("communicator", parentComm)
	state.Put("ctid", "100")

	step := &stepSetupContainerComm{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	// Check that communicator was replaced with pctExecCommunicator
	comm := state.Get("communicator")
	if _, ok := comm.(*pctExecCommunicator); !ok {
		t.Errorf("Expected communicator to be *pctExecCommunicator, got %T", comm)
	}

	// Check that proxmox_comm was set
	proxmoxComm := state.Get("proxmox_comm")
	if proxmoxComm != parentComm {
		t.Errorf("Expected proxmox_comm to be parentComm")
	}
}

func TestStepSetupContainerComm_Cleanup(t *testing.T) {
	step := &stepSetupContainerComm{}
	state := new(multistep.BasicStateBag)

	// Should not panic
	step.Cleanup(state)
}

// Test step interfaces are properly implemented
func TestSteps_ImplementStep(t *testing.T) {
	steps := []multistep.Step{
		&stepConnect{},
		&stepGetCTID{},
		&stepCreateContainer{},
		&stepMergeConfig{},
		&stepStartContainer{},
		&stepSetupContainerComm{},
		&stepBackupContainer{},
		&stepDestroyContainer{},
		&stepCreateTemplate{},
	}

	if len(steps) != 9 {
		t.Errorf("Expected 9 steps, got %d", len(steps))
	}
}

// someError implements error interface for testing
type someError struct {
	msg string
}

func (e *someError) Error() string {
	return e.msg
}

func TestStepCreateContainer_Cleanup_AlreadyDestroyed(t *testing.T) {
	// If container was already destroyed, cleanup should skip
	comm := &mockCommandRunner{}
	state := new(multistep.BasicStateBag)
	state.Put("ctid", "100")
	state.Put("communicator", comm)
	state.Put("container_destroyed", true)

	step := &stepCreateContainer{}
	step.Cleanup(state)

	// Cleanup should not call any commands since container was already destroyed
	if len(comm.calls) != 0 {
		t.Errorf("Expected no commands called, got %d calls", len(comm.calls))
	}
}

func TestStepCreateContainer_Cleanup_Reused(t *testing.T) {
	// If container was reused, cleanup should skip
	comm := &mockCommandRunner{}
	state := new(multistep.BasicStateBag)
	state.Put("ctid", "100")
	state.Put("communicator", comm)
	state.Put("container_reused", true)

	step := &stepCreateContainer{}
	step.Cleanup(state)

	// Cleanup should not call any commands since container was reused
	if len(comm.calls) != 0 {
		t.Errorf("Expected no commands called, got %d calls", len(comm.calls))
	}
}

func TestStepCreateContainer_Cleanup_Templated(t *testing.T) {
	// If container was successfully converted to a CT template, cleanup
	// should skip destroying it — the templated container is the artifact.
	comm := &mockCommandRunner{}
	state := new(multistep.BasicStateBag)
	state.Put("ctid", "100")
	state.Put("communicator", comm)
	state.Put("container_templated", true)

	step := &stepCreateContainer{}
	step.Cleanup(state)

	if len(comm.calls) != 0 {
		t.Errorf("Expected no commands called, got %d calls", len(comm.calls))
	}
}

func TestStepCreateContainer_Cleanup_NeedsCleanup(t *testing.T) {
	// If container was created but not destroyed, cleanup should destroy it
	comm := &mockCommandRunner{outputs: []string{"", "", ""}, errors: []error{nil, nil, nil}}
	state := new(multistep.BasicStateBag)
	state.Put("ctid", "100")
	state.Put("communicator", comm)
	state.Put("ui", &testUi{})
	// container_reused is false (or not set) and container_destroyed is not set

	step := &stepCreateContainer{}
	step.Cleanup(state)

	// Should call pct stop, pct destroy, and rm
	if len(comm.calls) < 3 {
		t.Errorf("Expected at least 3 commands called, got %d calls", len(comm.calls))
	}
}

func TestStepCreateContainer_Cleanup_NoCommunicator(t *testing.T) {
	// If communicator is not in state, cleanup should not panic
	state := new(multistep.BasicStateBag)
	state.Put("ctid", "100")
	// No communicator in state

	step := &stepCreateContainer{}
	step.Cleanup(state)
	// Should not panic
}

func TestStepCreateContainer_Cleanup_NoCtid(t *testing.T) {
	// If ctid is not in state, cleanup should not panic
	comm := &mockCommandRunner{}
	state := new(multistep.BasicStateBag)
	state.Put("communicator", comm)
	// No ctid in state

	step := &stepCreateContainer{}
	step.Cleanup(state)
	// Should not panic
}

func TestStepCreateContainer_Cleanup_WithProxmoxComm(t *testing.T) {
	// If proxmox_comm is set (after stepSetupContainerComm), cleanup should use it
	// This simulates a script failure during provisioning
	hostComm := &mockCommandRunner{outputs: []string{"", "", ""}, errors: []error{nil, nil, nil}}
	state := new(multistep.BasicStateBag)
	state.Put("ctid", "100")
	state.Put("proxmox_comm", hostComm)
	state.Put("ui", &testUi{})
	// container_reused is false and container_destroyed is not set

	step := &stepCreateContainer{}
	step.Cleanup(state)

	// Should call pct stop, pct destroy, and rm on the host communicator
	if len(hostComm.calls) < 3 {
		t.Errorf("Expected at least 3 commands called, got %d calls", len(hostComm.calls))
	}
}

func TestStepCreateContainer_Cleanup_ScriptFailure(t *testing.T) {
	// Simulate a script failing during provisioning
	// At this point, stepSetupContainerComm has run and set proxmox_comm
	hostComm := &mockCommandRunner{outputs: []string{"", "", ""}, errors: []error{nil, nil, nil}}
	state := new(multistep.BasicStateBag)
	state.Put("ctid", "100")
	state.Put("proxmox_comm", hostComm)
	state.Put("ui", &testUi{})
	// Simulate that container was not reused and not yet destroyed

	step := &stepCreateContainer{}
	step.Cleanup(state)

	// Verify the correct commands were run on the host
	expectedCommands := []string{
		fmt.Sprintf("pct stop %s || true", "100"),
		fmt.Sprintf("pct destroy %s --purge", "100"),
		fmt.Sprintf("rm -f /tmp/vzdump-lxc-%s-*.log 2>/dev/null || true", "100"),
	}
	if len(hostComm.calls) != len(expectedCommands) {
		t.Errorf("Expected %d commands, got %d", len(expectedCommands), len(hostComm.calls))
	}
	for i, expected := range expectedCommands {
		if i < len(hostComm.calls) && hostComm.calls[i] != expected {
			t.Errorf("Expected command %d to be %q, got %q", i, expected, hostComm.calls[i])
		}
	}
}

func TestStepCreateTemplate_Success(t *testing.T) {
	ui := &testUi{}
	comm := &mockCommandRunner{outputs: []string{"", ""}, errors: []error{nil, nil}}

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepCreateTemplate{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	templated, ok := state.Get("container_templated").(bool)
	if !ok || !templated {
		t.Errorf("Expected container_templated true")
	}

	expectedCommands := []string{"pct stop 100", "pct template 100"}
	if len(comm.calls) != len(expectedCommands) {
		t.Fatalf("Expected %d commands, got %d", len(expectedCommands), len(comm.calls))
	}
	for i, expected := range expectedCommands {
		if comm.calls[i] != expected {
			t.Errorf("Expected command %d to be %q, got %q", i, expected, comm.calls[i])
		}
	}
}

func TestStepCreateTemplate_StopError(t *testing.T) {
	ui := &testUi{}
	comm := &mockCommandRunner{errors: []error{&someError{msg: "pct stop failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepCreateTemplate{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if _, ok := state.GetOk("error"); !ok {
		t.Errorf("Expected error in state")
	}
	if _, ok := state.GetOk("container_templated"); ok {
		t.Errorf("Expected container_templated not set")
	}
}

func TestStepCreateTemplate_TemplateError(t *testing.T) {
	ui := &testUi{}
	comm := &mockCommandRunner{
		outputs: []string{"", ""},
		errors:  []error{nil, &someError{msg: "pct template failed"}},
	}

	state := new(multistep.BasicStateBag)
	state.Put("ui", ui)
	state.Put("proxmox_comm", comm)
	state.Put("ctid", "100")

	step := &stepCreateTemplate{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
	if _, ok := state.GetOk("error"); !ok {
		t.Errorf("Expected error in state")
	}
	if _, ok := state.GetOk("container_templated"); ok {
		t.Errorf("Expected container_templated not set")
	}
}

func TestStepCreateTemplate_Cleanup(t *testing.T) {
	step := &stepCreateTemplate{}
	state := new(multistep.BasicStateBag)

	// Should not panic
	step.Cleanup(state)
}
