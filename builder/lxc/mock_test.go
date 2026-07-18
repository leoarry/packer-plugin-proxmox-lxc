package lxc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// boolPtr returns a pointer to b, for populating *bool config fields in tests.
func boolPtr(b bool) *bool {
	return &b
}

// mockCommandRunner implements CommandRunner for testing.
// It returns a sequence of (output, error) pairs for successive commands.
type mockCommandRunner struct {
	outputs []string
	stderrs []string // optional, parallel to outputs; stderr content per call
	errors  []error
	idx     int
	calls   []string // Records the commands that were called
}

func (m *mockCommandRunner) RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	m.calls = append(m.calls, command) // Record the command
	if m.idx >= len(m.outputs) && m.idx >= len(m.errors) {
		return nil
	}

	var out, errOut string
	var err error
	if m.idx < len(m.outputs) {
		out = m.outputs[m.idx]
	}
	if m.idx < len(m.stderrs) {
		errOut = m.stderrs[m.idx]
	}
	if m.idx < len(m.errors) {
		err = m.errors[m.idx]
	}
	m.idx++

	if stdout != nil && out != "" {
		_, _ = stdout.Write([]byte(out))
	}
	if stderr != nil && errOut != "" {
		_, _ = stderr.Write([]byte(errOut))
	}
	return err
}

// mockCommunicator implements Communicator for testing.
// It records calls and returns configurable errors.
type mockCommunicator struct {
	commands    []string // Record of commands passed to Start()
	uploads     []string // Record of Upload calls
	errOnStrat  error    // Error to return on Start()
	errOnUpload error    // Error to return on Upload()
}

func (m *mockCommunicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
	m.commands = append(m.commands, cmd.Command)
	if m.errOnStrat != nil {
		return m.errOnStrat
	}
	cmd.SetExited(0)
	return nil
}

func (m *mockCommunicator) Upload(dst string, src io.Reader, fi *os.FileInfo) error {
	m.uploads = append(m.uploads, dst)
	return m.errOnUpload
}

func (m *mockCommunicator) UploadDir(dst string, src string, exclude []string) error {
	return nil
}

func (m *mockCommunicator) Download(src string, dst io.Writer) error {
	return nil
}

func (m *mockCommunicator) DownloadDir(src string, dst string, exclude []string) error {
	return nil
}

func (m *mockCommunicator) Wait(*packersdk.RemoteCmd) error {
	return nil
}

// mockPctExecCommunicator is a simplified pctExecCommunicator for testing.
// It uses a CommandRunner for the parent to allow mocking.
type mockPctExecCommunicator struct {
	ctid   string
	parent CommandRunner
}

func (m *mockPctExecCommunicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
	// Simulate pct exec command
	escaped := strings.ReplaceAll(cmd.Command, "'", "'\"'\"'")
	_ = fmt.Sprintf("pct exec %s -- bash -c '%s'", m.ctid, escaped)
	cmd.SetExited(0)
	return nil
}

func (m *mockPctExecCommunicator) Upload(dst string, src io.Reader, fi *os.FileInfo) error {
	// Simulate reading src and running pct push
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}
	_ = data
	err = m.parent.RunCommand(context.Background(), fmt.Sprintf("pct push %s %s %s", m.ctid, "/tmp/packer-upload", dst), nil, nil)
	return err
}

func (m *mockPctExecCommunicator) UploadDir(dst string, src string, exclude []string) error {
	return nil
}

func (m *mockPctExecCommunicator) Download(src string, dst io.Writer) error {
	return nil
}

func (m *mockPctExecCommunicator) DownloadDir(src string, dst string, exclude []string) error {
	return nil
}

func (m *mockPctExecCommunicator) Wait(*packersdk.RemoteCmd) error {
	return nil
}

// mockSSHSession implements SSHSession for testing.
type mockSSHSession struct {
	stdout    bytes.Buffer
	stderr    bytes.Buffer
	runErr    error
	output    []byte
	outputErr error
	stdoutW   io.Writer
	stderrW   io.Writer
}

func (s *mockSSHSession) Run(cmd string) error {
	if s.runErr != nil {
		return s.runErr
	}
	// Simulate writing to stdout
	if s.stdoutW != nil {
		_, _ = s.stdoutW.Write(s.output)
	} else {
		s.stdout.Write(s.output)
	}
	return nil
}

func (s *mockSSHSession) Output(cmd string) ([]byte, error) {
	return s.output, s.outputErr
}

func (s *mockSSHSession) SetStdout(w io.Writer) {
	s.stdoutW = w
}

func (s *mockSSHSession) SetStderr(w io.Writer) {
	s.stderrW = w
}

func (s *mockSSHSession) Close() error {
	return nil
}

// mockSSHSessionProvider implements SSHSessionProvider for testing.
type mockSSHSessionProvider struct {
	session SSHSession
	err     error
}

func (p *mockSSHSessionProvider) NewSession() (SSHSession, error) {
	return p.session, p.err
}

// Ensure mockSSHSessionProvider implements SSHSessionProvider.
var _ SSHSessionProvider = &mockSSHSessionProvider{}
