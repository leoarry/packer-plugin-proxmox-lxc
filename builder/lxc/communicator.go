package lxc

import (
	"bytes"
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
	Upload(dst string, src io.Reader, fi *os.FileInfo) error
	UploadDir(dst string, src string, exclude []string) error
	Download(src string, dst io.Writer) error
	DownloadDir(src string, dst string, exclude []string) error
}

// sshCommunicator executes commands on the Proxmox host via SSH.
type sshCommunicator struct {
	client SSHSessionProvider
}

// RunCommand executes a command on the Proxmox host and returns stdout.
// This method implements the CommandRunner interface.
func (c *sshCommunicator) RunCommand(ctx context.Context, command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer func() { _ = session.Close() }()

	var stdout bytes.Buffer
	session.SetStdout(&stdout)

	err = session.Run(command)
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			return stdout.String(), fmt.Errorf("command exited with status %d: %w", exitErr.ExitStatus(), err)
		}
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func (c *sshCommunicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer func() { _ = session.Close() }()

	var stdout, stderr bytes.Buffer
	session.SetStdout(&stdout)
	session.SetStderr(&stderr)

	command := cmd.Command

	err = session.Run(command)
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			cmd.SetExited(exitErr.ExitStatus())
		} else {
			cmd.SetExited(1)
		}
		return err
	}
	cmd.SetExited(0)
	return nil
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
}

func (c *pctExecCommunicator) Start(ctx context.Context, cmd *packersdk.RemoteCmd) error {
	escaped := strings.ReplaceAll(cmd.Command, "'", "'\"'\"'")
	pctCmd := fmt.Sprintf("pct exec %s -- bash -c '%s'", c.ctid, escaped)

	output, err := c.parent.RunCommand(ctx, pctCmd)
	_ = output // stdout captured but not used directly
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			cmd.SetExited(exitErr.ExitStatus())
		} else {
			cmd.SetExited(1)
		}
		return err
	}
	cmd.SetExited(0)
	return nil
}

func (c *pctExecCommunicator) Upload(dst string, src io.Reader, fi *os.FileInfo) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("failed to read upload source: %w", err)
	}
	tmpFile := fmt.Sprintf("/tmp/packer-upload-%s", c.ctid)
	writeCmd := fmt.Sprintf("cat > %s << 'PACKEREOF'\n%s\nPACKEREOF", tmpFile, string(data))
	_, err = c.parent.RunCommand(context.Background(), writeCmd)
	if err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	_, err = c.parent.RunCommand(context.Background(), fmt.Sprintf("pct push %s %s %s", c.ctid, tmpFile, dst))
	_, _ = c.parent.RunCommand(context.Background(), fmt.Sprintf("rm -f %s", tmpFile))
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
