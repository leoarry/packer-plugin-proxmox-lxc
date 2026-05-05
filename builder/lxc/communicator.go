package lxc

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

// Communicator interface defines the methods for communicating with Proxmox host and containers.
// Both sshCommunicator and pctExecCommunicator implement this interface.
type Communicator interface {
	Start(ctx context.Context, cmd *packersdk.RemoteCmd) error
	Wait(*packersdk.RemoteCmd) error
	Upload(dst string, src io.Reader, fi *os.FileInfo) error
	UploadDir(dst string, src string, exclude []string) error
	Download(src string, dst io.Writer) error
	DownloadDir(src string, dst string, exclude []string) error
}

// sshCommunicator executes commands on the Proxmox host via SSH.
type sshCommunicator struct {
	client SSHSessionProvider
	cmdErr error
	done   chan struct{}
}

// RunCommand executes a command on the Proxmox host.
// This method implements the CommandRunner interface.
func (c *sshCommunicator) RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
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

func (c *sshCommunicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
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

func (c *sshCommunicator) Wait(*packersdk.RemoteCmd) error {
	<-c.done
	return c.cmdErr
}

func (c *sshCommunicator) Upload(dst string, src io.Reader, fi *os.FileInfo) error {
	return fmt.Errorf("Upload not implemented, use pct push instead")
}

func (c *sshCommunicator) UploadDir(dst string, src string, exclude []string) error {
	return fmt.Errorf("UploadDir not implemented")
}

func (c *sshCommunicator) Download(src string, dst io.Writer) error {
	return fmt.Errorf("Download not implemented")
}

func (c *sshCommunicator) DownloadDir(src string, dst string, exclude []string) error {
	return fmt.Errorf("DownloadDir not implemented")
}

// pctExecCommunicator runs commands inside an LXC container via pct exec.
type pctExecCommunicator struct {
	ctid   string
	parent CommandRunner
	cmdErr error
	done   chan struct{}
}

func (c *pctExecCommunicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
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

func (c *pctExecCommunicator) Wait(*packersdk.RemoteCmd) error {
	<-c.done
	return c.cmdErr
}

func (c *pctExecCommunicator) Upload(dst string, src io.Reader, fi *os.FileInfo) error {
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

func (c *pctExecCommunicator) UploadDir(dst string, src string, exclude []string) error {
	return fmt.Errorf("UploadDir not implemented for pct exec communicator")
}

func (c *pctExecCommunicator) Download(src string, dst io.Writer) error {
	return fmt.Errorf("Download not implemented for pct exec communicator")
}

func (c *pctExecCommunicator) DownloadDir(src string, dst string, exclude []string) error {
	return fmt.Errorf("DownloadDir not implemented for pct exec communicator")
}
