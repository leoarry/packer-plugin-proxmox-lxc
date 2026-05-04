package lxc

import (
	"io"
	"golang.org/x/crypto/ssh"
)

// SSHSession abstracts an SSH session for testability.
type SSHSession interface {
	// Run runs cmd on the remote host, returning its combined stdout and stderr.
	Run(cmd string) error
	// Output runs cmd and returns its stdout.
	Output(cmd string) ([]byte, error)
	// Stdout and Stderr pipes
	SetStdout(io.Writer)
	SetStderr(io.Writer)
	Close() error
}

// sshSessionWrapper wraps *ssh.Session to implement SSHSession.
type sshSessionWrapper struct {
	session *ssh.Session
}

func (s *sshSessionWrapper) Run(cmd string) error {
	return s.session.Run(cmd)
}

func (s *sshSessionWrapper) Output(cmd string) ([]byte, error) {
	return s.session.Output(cmd)
}

func (s *sshSessionWrapper) SetStdout(w io.Writer) {
	s.session.Stdout = w
}

func (s *sshSessionWrapper) SetStderr(w io.Writer) {
	s.session.Stderr = w
}

func (s *sshSessionWrapper) Close() error {
	return s.session.Close()
}

// SSHSessionProvider abstracts SSH session creation for testability.
type SSHSessionProvider interface {
	NewSession() (SSHSession, error)
}

// sshClientWrapper wraps *ssh.Client to implement SSHSessionProvider.
type sshClientWrapper struct {
	client *ssh.Client
}

func (w *sshClientWrapper) NewSession() (SSHSession, error) {
	session, err := w.client.NewSession()
	if err != nil {
		return nil, err
	}
	return &sshSessionWrapper{session: session}, nil
}

// Ensure sshClientWrapper implements SSHSessionProvider.
var _ SSHSessionProvider = &sshClientWrapper{}
