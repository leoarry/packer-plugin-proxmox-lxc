// Package remote runs commands on the Proxmox host over SSH, and inside LXC
// containers on it via pct exec.
package remote

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

// CommandRunner runs a single command and streams its output. It is implemented
// by SSHCommunicator and can be mocked for testing.
type CommandRunner interface {
	RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error
}

// Communicator interface defines the methods for communicating with Proxmox host and containers.
// Both SSHCommunicator and PctExecCommunicator implement this interface.
type Communicator interface {
	Start(ctx context.Context, cmd *packersdk.RemoteCmd) error
	Wait(*packersdk.RemoteCmd) error
	Upload(dst string, src io.Reader, fi *os.FileInfo) error
	UploadDir(dst string, src string, exclude []string) error
	Download(src string, dst io.Writer) error
	DownloadDir(src string, dst string, exclude []string) error
}

// SSHCommunicator executes commands on the Proxmox host via SSH.
type SSHCommunicator struct {
	client SSHSessionProvider
	cmdErr error
	done   chan struct{}
}

// NewSSHCommunicator returns a communicator that runs commands on the host
// behind client.
func NewSSHCommunicator(client *ssh.Client) *SSHCommunicator {
	return &SSHCommunicator{client: &sshClientWrapper{client: client}}
}

// RunCommand executes a command on the Proxmox host.
// This method implements the CommandRunner interface.
func (c *SSHCommunicator) RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer func() { _ = session.Close() }()

	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	session.SetStdout(stdout)
	session.SetStderr(stderr)

	err = session.Run(command)
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return fmt.Errorf("command exited with status %d: %w", exitErr.ExitStatus(), err)
		}
		return err
	}
	return nil
}

func (c *SSHCommunicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
	c.cmdErr = nil
	c.done = make(chan struct{})

	go func() {
		defer close(c.done)
		session, err := c.client.NewSession()
		if err != nil {
			c.cmdErr = fmt.Errorf("failed to create SSH session: %w", err)
			cmd.SetExited(1)
			return
		}
		defer func() { _ = session.Close() }()

		if cmd.Stdout != nil {
			session.SetStdout(cmd.Stdout)
		} else {
			session.SetStdout(io.Discard)
		}
		if cmd.Stderr != nil {
			session.SetStderr(cmd.Stderr)
		} else {
			session.SetStderr(io.Discard)
		}

		err = session.Run(cmd.Command)
		c.cmdErr = err
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				cmd.SetExited(exitErr.ExitStatus())
			} else {
				cmd.SetExited(1)
			}
		} else {
			cmd.SetExited(0)
		}
	}()

	return nil
}

func (c *SSHCommunicator) Wait(*packersdk.RemoteCmd) error {
	<-c.done
	return c.cmdErr
}

func (c *SSHCommunicator) Upload(dst string, src io.Reader, fi *os.FileInfo) error {
	return fmt.Errorf("Upload not implemented, use pct push instead")
}

func (c *SSHCommunicator) UploadDir(dst string, src string, exclude []string) error {
	return fmt.Errorf("UploadDir not implemented")
}

func (c *SSHCommunicator) Download(src string, dst io.Writer) error {
	return fmt.Errorf("Download not implemented")
}

func (c *SSHCommunicator) DownloadDir(src string, dst string, exclude []string) error {
	return fmt.Errorf("DownloadDir not implemented")
}

// PctExecCommunicator runs commands inside an LXC container via pct exec.
type PctExecCommunicator struct {
	ctid   string
	parent CommandRunner
	cmdErr error
	done   chan struct{}
}

// NewPctExecCommunicator returns a communicator that runs commands inside
// container ctid, through parent (a runner on the Proxmox host).
func NewPctExecCommunicator(ctid string, parent CommandRunner) *PctExecCommunicator {
	return &PctExecCommunicator{ctid: ctid, parent: parent}
}

func (c *PctExecCommunicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
	escaped := strings.ReplaceAll(cmd.Command, "'", "'\"'\"'")
	pctCmd := fmt.Sprintf("pct exec %s -- bash -c '%s'", c.ctid, escaped)

	c.cmdErr = nil
	c.done = make(chan struct{})

	go func() {
		defer close(c.done)
		err := c.parent.RunCommand(ctx, pctCmd, cmd.Stdout, cmd.Stderr)
		c.cmdErr = err
		if err != nil {
			if exitErr, ok := err.(*ssh.ExitError); ok {
				cmd.SetExited(exitErr.ExitStatus())
			} else {
				cmd.SetExited(1)
			}
		} else {
			cmd.SetExited(0)
		}
	}()

	return nil
}

func (c *PctExecCommunicator) Wait(*packersdk.RemoteCmd) error {
	<-c.done
	return c.cmdErr
}

func (c *PctExecCommunicator) Upload(dst string, src io.Reader, fi *os.FileInfo) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("failed to read upload source: %w", err)
	}
	tmpFile := fmt.Sprintf("/tmp/packer-upload-%s", c.ctid)
	writeCmd := fmt.Sprintf("cat > %s << 'PACKEREOF'\n%s\nPACKEREOF", tmpFile, string(data))
	err = c.parent.RunCommand(context.Background(), writeCmd, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	err = c.parent.RunCommand(context.Background(), fmt.Sprintf("pct push %s %s %s", c.ctid, tmpFile, dst), nil, nil)
	_ = c.parent.RunCommand(context.Background(), fmt.Sprintf("rm -f %s", tmpFile), nil, nil)
	if err != nil {
		return fmt.Errorf("pct push failed: %w", err)
	}
	return nil
}

func (c *PctExecCommunicator) UploadDir(dst string, src string, exclude []string) error {
	return fmt.Errorf("UploadDir not implemented for pct exec communicator")
}

func (c *PctExecCommunicator) Download(src string, dst io.Writer) error {
	return fmt.Errorf("Download not implemented for pct exec communicator")
}

func (c *PctExecCommunicator) DownloadDir(src string, dst string, exclude []string) error {
	return fmt.Errorf("DownloadDir not implemented for pct exec communicator")
}
