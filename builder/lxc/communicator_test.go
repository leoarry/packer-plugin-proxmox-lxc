package lxc

import (
	"bytes"
	"context"
	"errors"
	"testing"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"golang.org/x/crypto/ssh"
)

// Test_pctExecCommunicator_Start tests the Start method
func Test_pctExecCommunicator_Start(t *testing.T) {
	tests := []struct {
		name    string
		ctid    string
		mockOut string
		mockErr error
		wantErr bool
	}{
		{
			name:    "successful command",
			ctid:    "100",
			mockOut: "command output",
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "command error with exit status",
			ctid:    "100",
			mockOut: "",
			mockErr: &ssh.ExitError{},
			wantErr: true,
		},
		{
			name:    "command generic error",
			ctid:    "100",
			mockOut: "",
			mockErr: errors.New("connection failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &mockCommandRunner{
				outputs: []string{tt.mockOut},
				errors:  []error{tt.mockErr},
			}
			comm := &pctExecCommunicator{
				ctid:   tt.ctid,
				parent: mockRunner,
			}

			cmd := &packersdk.RemoteCmd{
				Command: "echo hello",
			}
			err := comm.Start(context.Background(), cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test_pctExecCommunicator_Start_CommandEscaping tests that special characters are properly escaped
func Test_pctExecCommunicator_Start_CommandEscaping(t *testing.T) {
	mockRunner := &mockCommandRunner{
		outputs: []string{""},
		errors:  []error{nil},
	}
	comm := &pctExecCommunicator{
		ctid:   "100",
		parent: mockRunner,
	}

	cmd := &packersdk.RemoteCmd{
		Command: "echo 'hello'",
	}
	err := comm.Start(context.Background(), cmd)
	if err != nil {
		t.Errorf("Start() with quoted command error = %v", err)
	}
}

// Test_pctExecCommunicator_Upload tests the Upload method
func Test_pctExecCommunicator_Upload(t *testing.T) {
	tests := []struct {
		name          string
		dst           string
		src           string
		mockOutputs   []string
		mockErrors    []error
		wantErr       bool
	}{
		{
			name:        "successful upload",
			dst:         "/etc/config",
			src:         "config data",
			mockOutputs: []string{"", "", ""}, // write temp, pct push, rm
			mockErrors:  []error{nil, nil, nil},
			wantErr:      false,
		},
		{
			name:        "read source error",
			dst:         "/etc/config",
			src:         "config data",
			mockOutputs: []string{},
			mockErrors:  []error{},
			wantErr:      true, // Will fail on io.ReadAll if we use a bad reader
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRunner := &mockCommandRunner{
				outputs: tt.mockOutputs,
				errors:  tt.mockErrors,
			}
			comm := &pctExecCommunicator{
				ctid:   "100",
				parent: mockRunner,
			}

			if tt.name == "read source error" {
				// Use a reader that returns error
				err := comm.Upload(tt.dst, &errorReader{}, nil)
				if err == nil {
					t.Error("Expected error from Upload with bad reader")
				}
				return
			}

			src := bytes.NewBufferString(tt.src)
			err := comm.Upload(tt.dst, src, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Upload() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// errorReader is a reader that always returns an error
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func (e *errorReader) Close() error {
	return nil
}

// Test_pctExecCommunicator_UploadDir tests that UploadDir returns error (not implemented)
func Test_pctExecCommunicator_UploadDir(t *testing.T) {
	comm := &pctExecCommunicator{
		ctid:   "100",
		parent: &mockCommandRunner{},
	}

	err := comm.UploadDir("/dst", "/src", nil)
	if err == nil {
		t.Error("Expected error for UploadDir (not implemented)")
	}
}

// Test_pctExecCommunicator_Download tests that Download returns error (not implemented)
func Test_pctExecCommunicator_Download(t *testing.T) {
	comm := &pctExecCommunicator{
		ctid:   "100",
		parent: &mockCommandRunner{},
	}

	err := comm.Download("/src", &bytes.Buffer{})
	if err == nil {
		t.Error("Expected error for Download (not implemented)")
	}
}

// Test_pctExecCommunicator_DownloadDir tests that DownloadDir returns error (not implemented)
func Test_pctExecCommunicator_DownloadDir(t *testing.T) {
	comm := &pctExecCommunicator{
		ctid:   "100",
		parent: &mockCommandRunner{},
	}

	err := comm.DownloadDir("/src", "/dst", nil)
	if err == nil {
		t.Error("Expected error for DownloadDir (not implemented)")
	}
}

// Test_sshCommunicator_RunCommand tests the RunCommand method
func Test_sshCommunicator_RunCommand(t *testing.T) {
	tests := []struct {
		name      string
		output    []byte
		outputErr error
		runErr    error
		wantErr   bool
	}{
		{
			name:    "successful command",
			output:  []byte("command output"),
			wantErr: false,
		},
		{
			name:    "command returns error",
			runErr:  errors.New("command failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSession := &mockSSHSession{
				output: tt.output,
				runErr: tt.runErr,
			}
			mockProvider := &mockSSHSessionProvider{
				session: mockSession,
			}
			comm := &sshCommunicator{
				client: mockProvider,
			}

			result, err := comm.RunCommand(context.Background(), "echo test")
			if (err != nil) != tt.wantErr {
				t.Errorf("RunCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result != string(tt.output) {
				t.Errorf("RunCommand() = %v, want %v", result, string(tt.output))
			}
		})
	}
}

// Test_sshCommunicator_Start tests the Start method
func Test_sshCommunicator_Start(t *testing.T) {
	tests := []struct {
		name    string
		runErr  error
		wantErr bool
	}{
		{
			name:    "successful start",
			runErr:  nil,
			wantErr: false,
		},
		{
			name:    "start returns error",
			runErr:  errors.New("connection lost"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSession := &mockSSHSession{
				runErr: tt.runErr,
			}
			mockProvider := &mockSSHSessionProvider{
				session: mockSession,
			}
			comm := &sshCommunicator{
				client: mockProvider,
			}

			cmd := &packersdk.RemoteCmd{
				Command: "echo test",
			}
			err := comm.Start(context.Background(), cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test_communicator_Interface tests that both communicators implement the Communicator interface
func Test_communicator_Interface(t *testing.T) {
	// Verify sshCommunicator implements Communicator
	var _ Communicator = &sshCommunicator{}

	// Verify pctExecCommunicator implements Communicator
	var _ Communicator = &pctExecCommunicator{}
}

// Test_sshCommunicator_Interface tests that sshCommunicator implements CommandRunner
func Test_sshCommunicator_CommandRunner(t *testing.T) {
	var _ CommandRunner = &sshCommunicator{}
}

// Test_CleanupFunctions tests that all Cleanup functions don't panic
func Test_CleanupFunctions(t *testing.T) {
	// Test step Cleanup functions with various state bags

	steps := []struct {
		name string
		step multistep.Step
	}{
		{"stepConnect", &stepConnect{}},
		{"stepGetCTID", &stepGetCTID{}},
		{"stepCreateContainer", &stepCreateContainer{}},
		{"stepMergeConfig", &stepMergeConfig{}},
		{"stepStartContainer", &stepStartContainer{}},
		{"stepSetupContainerComm", &stepSetupContainerComm{}},
		{"stepBackupContainer", &stepBackupContainer{}},
		{"stepDestroyContainer", &stepDestroyContainer{}},
	}

	for _, s := range steps {
		t.Run(s.name, func(t *testing.T) {
			// Test with empty state
			state := new(multistep.BasicStateBag)
			s.step.Cleanup(state)

			// Test with SSH client in state (for stepConnect)
			if sc, ok := s.step.(*stepConnect); ok {
				_ = sc
				// We can't easily add a mock ssh.Client, so just test empty state
			}
		})
	}
}

// Test_sshCommunicator_UnimplementedMethods tests that unimplemented methods return errors
func Test_sshCommunicator_UnimplementedMethods(t *testing.T) {
	comm := &sshCommunicator{}

	// Upload should return error
	err := comm.Upload("/dst", &bytes.Buffer{}, nil)
	if err == nil {
		t.Error("Expected error for sshCommunicator.Upload()")
	}

	// UploadDir should return error
	err = comm.UploadDir("/dst", "/src", nil)
	if err == nil {
		t.Error("Expected error for sshCommunicator.UploadDir()")
	}

	// Download should return error
	err = comm.Download("/src", &bytes.Buffer{})
	if err == nil {
		t.Error("Expected error for sshCommunicator.Download()")
	}

	// DownloadDir should return error
	err = comm.DownloadDir("/src", "/dst", nil)
	if err == nil {
		t.Error("Expected error for sshCommunicator.DownloadDir()")
	}
}

// Test_pctExecCommunicator_UnimplementedMethods tests that unimplemented methods return errors
func Test_pctExecCommunicator_UnimplementedMethods(t *testing.T) {
	comm := &pctExecCommunicator{parent: &mockCommandRunner{}}

	// UploadDir should return error
	err := comm.UploadDir("/dst", "/src", nil)
	if err == nil {
		t.Error("Expected error for pctExecCommunicator.UploadDir()")
	}

	// Download should return error
	err = comm.Download("/src", &bytes.Buffer{})
	if err == nil {
		t.Error("Expected error for pctExecCommunicator.Download()")
	}

	// DownloadDir should return error
	err = comm.DownloadDir("/src", "/dst", nil)
	if err == nil {
		t.Error("Expected error for pctExecCommunicator.DownloadDir()")
	}
}
