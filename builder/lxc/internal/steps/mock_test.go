package steps

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

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

// mockPctExecCommunicator is a simplified remote.PctExecCommunicator for testing.
// It uses a remote.CommandRunner for the parent to allow mocking.
type mockPctExecCommunicator struct {
	ctid   string
	parent remote.CommandRunner
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

// someError implements error interface for testing
type someError struct {
	msg string
}

func (e *someError) Error() string {
	return e.msg
}
