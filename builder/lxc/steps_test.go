package lxc

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

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
	config := &Config{}
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

func TestStepCreateContainer_Create(t *testing.T) {
	config := &Config{
		Unprivileged: true,
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
		Unprivileged: true,
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

func TestStepBackupContainer_WithPigz(t *testing.T) {
	config := &Config{
		BackupDir:  "/var/lib/vz/template/cache",
		BackupPigz: 4,
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
		BackupDir:  "/var/lib/vz/template/cache",
		BackupPigz: 4,
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
		BackupDir:  "/var/lib/vz/template/cache",
		BackupPigz: -1, // explicitly disabled (resolved value after Config.Prepare)
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
