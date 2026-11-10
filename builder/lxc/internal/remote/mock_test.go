package remote

import (
	"bytes"
	"io"
)

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
