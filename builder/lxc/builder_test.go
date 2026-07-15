package lxc

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestBuilder_Implements(t *testing.T) {
	var _ packersdk.Builder = &Builder{}
}

func TestBuilder_Prepare(t *testing.T) {
	tests := []struct {
		name    string
		raw     []interface{}
		wantErr bool
	}{
		{
			name: "valid config with password",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_host":     "192.168.1.100",
					"ssh_port":     22,
					"ssh_user":     "root@pam",
					"ssh_password": "secret",
					"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
					"storage":      "local-lvm",
					"memory":       1024,
					"cores":        2,
					"rootfs_size":  "2",
					"ssh_timeout":  "5m",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with ssh key",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_host":     "192.168.1.100",
					"ssh_port":     22,
					"ssh_user":     "root@pam",
					"ssh_key_path": "/path/to/key",
					"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
					"rootfs_size":  "2",
				},
			},
			wantErr: false,
		},
		{
			name: "missing required ssh_host",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_user":     "root@pam",
					"ssh_password": "secret",
					"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
					"rootfs_size":  "2",
				},
			},
			wantErr: true,
		},
		{
			name: "missing required ssh_user",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_host":     "192.168.1.100",
					"ssh_password": "secret",
					"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
					"rootfs_size":  "2",
				},
			},
			wantErr: true,
		},
		{
			name: "missing auth credentials",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_host":    "192.168.1.100",
					"ssh_user":    "root@pam",
					"template":    "local:vztmpl/ubuntu-22.04.tar.gz",
					"rootfs_size": "2",
				},
			},
			wantErr: true,
		},
		{
			name: "missing template",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_host":     "192.168.1.100",
					"ssh_user":     "root@pam",
					"ssh_password": "secret",
					"rootfs_size":  "2",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port - negative",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_host":     "192.168.1.100",
					"ssh_port":     -1,
					"ssh_user":     "root@pam",
					"ssh_password": "secret",
					"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
					"rootfs_size":  "2",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid memory - negative",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_host":     "192.168.1.100",
					"ssh_user":     "root@pam",
					"ssh_password": "secret",
					"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
					"memory":       -1,
					"rootfs_size":  "2",
				},
			},
			wantErr: true,
		},
		{
			name: "multiple raw configs",
			raw: []interface{}{
				map[string]interface{}{
					"ssh_host": "192.168.1.100",
					"ssh_user": "root@pam",
				},
				map[string]interface{}{
					"ssh_password": "secret",
					"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
					"rootfs_size":  "2",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Builder{}
			_, _, err := b.Prepare(tt.raw...)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.Prepare() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuilder_ConfigSpec(t *testing.T) {
	b := &Builder{}
	spec := b.ConfigSpec()

	if spec == nil {
		t.Fatal("ConfigSpec() returned nil")
	}
}

// Mock steps for testing Builder.Run()

type mockStepConnect struct {
	action multistep.StepAction
}

func (s *mockStepConnect) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	state.Put("communicator", &mockCommandRunner{})
	state.Put("proxmox_comm", &mockCommandRunner{})
	return s.action
}
func (s *mockStepConnect) Cleanup(state multistep.StateBag) {}

type mockStepGetCTID struct {
	action multistep.StepAction
	ctid   string
}

func (s *mockStepGetCTID) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	state.Put("ctid", s.ctid)
	return s.action
}
func (s *mockStepGetCTID) Cleanup(state multistep.StateBag) {}

type mockStepCreateContainer struct {
	action multistep.StepAction
}

func (s *mockStepCreateContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	return s.action
}
func (s *mockStepCreateContainer) Cleanup(state multistep.StateBag) {}

type mockStepMergeConfig struct {
	action multistep.StepAction
}

func (s *mockStepMergeConfig) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	return s.action
}
func (s *mockStepMergeConfig) Cleanup(state multistep.StateBag) {}

type mockStepStartContainer struct {
	action multistep.StepAction
}

func (s *mockStepStartContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	return s.action
}
func (s *mockStepStartContainer) Cleanup(state multistep.StateBag) {}

type mockStepSetupContainerComm struct {
	action multistep.StepAction
}

func (s *mockStepSetupContainerComm) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	// Replace communicator with pctExecCommunicator
	parentComm := state.Get("communicator").(CommandRunner)
	state.Put("communicator", &pctExecCommunicator{ctid: "100", parent: parentComm})
	state.Put("proxmox_comm", parentComm)
	return s.action
}
func (s *mockStepSetupContainerComm) Cleanup(state multistep.StateBag) {}

type mockStepProvision struct {
	action multistep.StepAction
}

func (s *mockStepProvision) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	return s.action
}
func (s *mockStepProvision) Cleanup(state multistep.StateBag) {}

type mockStepBackupContainer struct {
	action     multistep.StepAction
	backupPath string
	backupName string
}

func (s *mockStepBackupContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.backupPath != "" {
		state.Put("backup_path", s.backupPath)
	}
	if s.backupName != "" {
		state.Put("backup_name", s.backupName)
	}
	return s.action
}
func (s *mockStepBackupContainer) Cleanup(state multistep.StateBag) {}

type mockStepCreateTemplate struct {
	action    multistep.StepAction
	templated bool
}

func (s *mockStepCreateTemplate) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.templated {
		state.Put("container_templated", true)
	}
	return s.action
}
func (s *mockStepCreateTemplate) Cleanup(state multistep.StateBag) {}

type mockStepDestroyContainer struct {
	action multistep.StepAction
}

func (s *mockStepDestroyContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	return s.action
}
func (s *mockStepDestroyContainer) Cleanup(state multistep.StateBag) {}

// errorStep is a mock step that puts an error in state
type errorStep struct{}

func (s *errorStep) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	state.Put("error", context.Canceled)
	return multistep.ActionContinue
}
func (s *errorStep) Cleanup(state multistep.StateBag) {}

func TestBuilder_Run_Success(t *testing.T) {
	b := &Builder{}
	_, _, err := b.Prepare(map[string]interface{}{
		"ssh_host":     "192.168.1.100",
		"ssh_user":     "root@pam",
		"ssh_password": "secret",
		"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
		"backup_dir":   "/var/lib/vz/template/cache",
		"rootfs_size":  "2",
	})
	if err != nil {
		t.Fatalf("Prepare() failed: %v", err)
	}

	mockSteps := []multistep.Step{
		&mockStepConnect{action: multistep.ActionContinue},
		&mockStepGetCTID{action: multistep.ActionContinue, ctid: "100"},
		&mockStepCreateContainer{action: multistep.ActionContinue},
		&mockStepMergeConfig{action: multistep.ActionContinue},
		&mockStepStartContainer{action: multistep.ActionContinue},
		&mockStepSetupContainerComm{action: multistep.ActionContinue},
		&mockStepProvision{action: multistep.ActionContinue},
		&mockStepBackupContainer{action: multistep.ActionContinue, backupPath: "/var/lib/vz/template/cache/lxc-template-100.tar.gz", backupName: "lxc-template-100"},
		&mockStepDestroyContainer{action: multistep.ActionContinue},
	}
	b.setSteps(mockSteps)

	ui := &testUi{}
	artifact, err := b.Run(context.Background(), ui, nil)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if artifact == nil {
		t.Fatal("Expected artifact, got nil")
	}

	a, ok := artifact.(*Artifact)
	if !ok {
		t.Fatalf("Expected *Artifact, got %T", artifact)
	}

	if a.BackupPath != "/var/lib/vz/template/cache/lxc-template-100.tar.gz" {
		t.Errorf("Expected backup path '/var/lib/vz/template/cache/lxc-template-100.tar.gz', got '%s'", a.BackupPath)
	}
}

func TestBuilder_Run_TemplateMethod(t *testing.T) {
	b := &Builder{}
	_, _, err := b.Prepare(map[string]interface{}{
		"ssh_host":      "192.168.1.100",
		"ssh_user":      "root@pam",
		"ssh_password":  "secret",
		"template":      "local:vztmpl/ubuntu-22.04.tar.gz",
		"rootfs_size":   "2",
		"backup_method": "template",
	})
	if err != nil {
		t.Fatalf("Prepare() failed: %v", err)
	}

	mockSteps := []multistep.Step{
		&mockStepConnect{action: multistep.ActionContinue},
		&mockStepGetCTID{action: multistep.ActionContinue, ctid: "100"},
		&mockStepCreateContainer{action: multistep.ActionContinue},
		&mockStepMergeConfig{action: multistep.ActionContinue},
		&mockStepStartContainer{action: multistep.ActionContinue},
		&mockStepSetupContainerComm{action: multistep.ActionContinue},
		&mockStepProvision{action: multistep.ActionContinue},
		&mockStepCreateTemplate{action: multistep.ActionContinue, templated: true},
	}
	b.setSteps(mockSteps)

	ui := &testUi{}
	artifact, err := b.Run(context.Background(), ui, nil)
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	a, ok := artifact.(*Artifact)
	if !ok {
		t.Fatalf("Expected *Artifact, got %T", artifact)
	}
	if a.Method != "template" {
		t.Errorf("Expected Method 'template', got %q", a.Method)
	}
	if a.CTID != "100" {
		t.Errorf("Expected CTID '100', got %q", a.CTID)
	}
}

func TestBuilder_Run_TemplateMethod_NotTemplated(t *testing.T) {
	b := &Builder{}
	_, _, err := b.Prepare(map[string]interface{}{
		"ssh_host":      "192.168.1.100",
		"ssh_user":      "root@pam",
		"ssh_password":  "secret",
		"template":      "local:vztmpl/ubuntu-22.04.tar.gz",
		"rootfs_size":   "2",
		"backup_method": "template",
	})
	if err != nil {
		t.Fatalf("Prepare() failed: %v", err)
	}

	mockSteps := []multistep.Step{
		&mockStepConnect{action: multistep.ActionContinue},
		&mockStepGetCTID{action: multistep.ActionContinue, ctid: "100"},
		&mockStepCreateContainer{action: multistep.ActionContinue},
		&mockStepMergeConfig{action: multistep.ActionContinue},
		&mockStepStartContainer{action: multistep.ActionContinue},
		&mockStepSetupContainerComm{action: multistep.ActionContinue},
		&mockStepProvision{action: multistep.ActionContinue},
		&mockStepCreateTemplate{action: multistep.ActionContinue, templated: false},
	}
	b.setSteps(mockSteps)

	ui := &testUi{}
	_, err = b.Run(context.Background(), ui, nil)
	if err == nil {
		t.Fatal("Expected error for container not templated, got nil")
	}
}

func TestBuilder_Run_NoBackupPath(t *testing.T) {
	b := &Builder{}
	_, _, err := b.Prepare(map[string]interface{}{
		"ssh_host":     "192.168.1.100",
		"ssh_user":     "root@pam",
		"ssh_password": "secret",
		"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
		"rootfs_size":  "2",
	})
	if err != nil {
		t.Fatalf("Prepare() failed: %v", err)
	}

	mockSteps := []multistep.Step{
		&mockStepConnect{action: multistep.ActionContinue},
		&mockStepGetCTID{action: multistep.ActionContinue, ctid: "100"},
		&mockStepCreateContainer{action: multistep.ActionContinue},
		&mockStepMergeConfig{action: multistep.ActionContinue},
		&mockStepStartContainer{action: multistep.ActionContinue},
		&mockStepSetupContainerComm{action: multistep.ActionContinue},
		&mockStepProvision{action: multistep.ActionContinue},
		&mockStepBackupContainer{action: multistep.ActionContinue}, // no backupPath set
		&mockStepDestroyContainer{action: multistep.ActionContinue},
	}
	b.setSteps(mockSteps)

	ui := &testUi{}
	_, err = b.Run(context.Background(), ui, nil)
	if err == nil {
		t.Fatal("Expected error for missing backup_path, got nil")
	}

	expectedErr := "no backup path found in state"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}
}

func TestBuilder_Run_ErrorInState(t *testing.T) {
	b := &Builder{}
	_, _, err := b.Prepare(map[string]interface{}{
		"ssh_host":     "192.168.1.100",
		"ssh_user":     "root@pam",
		"ssh_password": "secret",
		"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
		"rootfs_size":  "2",
	})
	if err != nil {
		t.Fatalf("Prepare() failed: %v", err)
	}

	mockSteps := []multistep.Step{
		&mockStepConnect{action: multistep.ActionContinue},
		&mockStepGetCTID{action: multistep.ActionHalt, ctid: "100"}, // Halt early
	}
	b.setSteps(mockSteps)

	ui := &testUi{}
	artifact, err := b.Run(context.Background(), ui, nil)
	if artifact != nil {
		t.Errorf("Expected nil artifact on error, got %v", artifact)
	}
	// err will be nil because the runner completes without putting error in state
	// The runner just stops when it gets ActionHalt
	_ = err
}

func TestBuilder_Run_WithErrorInState(t *testing.T) {
	// Test that Run returns error when error is in state
	b := &Builder{}
	_, _, err := b.Prepare(map[string]interface{}{
		"ssh_host":     "192.168.1.100",
		"ssh_user":     "root@pam",
		"ssh_password": "secret",
		"template":     "local:vztmpl/ubuntu-22.04.tar.gz",
		"rootfs_size":  "2",
	})
	if err != nil {
		t.Fatalf("Prepare() failed: %v", err)
	}

	b.setSteps([]multistep.Step{&errorStep{}})

	ui := &testUi{}
	_, err = b.Run(context.Background(), ui, nil)
	if err == nil {
		t.Fatal("Expected error from state, got nil")
	}
}
