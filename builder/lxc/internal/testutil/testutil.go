// Package testutil holds test doubles shared by the builder's packages. Only
// tests import it, so none of it ends up in the plugin binary.
package testutil

import (
	"context"
	"io"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// MockRunner implements remote.CommandRunner. Each call replays the next
// entry of Outputs, Stderrs and Errors (missing entries mean empty output and
// a nil error) and records the command in Calls.
type MockRunner struct {
	Outputs []string
	Stderrs []string // optional, parallel to Outputs
	Errors  []error
	Calls   []string

	idx int
}

func (m *MockRunner) RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	m.Calls = append(m.Calls, command)
	if m.idx >= len(m.Outputs) && m.idx >= len(m.Errors) {
		return nil
	}

	var out, errOut string
	var err error
	if m.idx < len(m.Outputs) {
		out = m.Outputs[m.idx]
	}
	if m.idx < len(m.Stderrs) {
		errOut = m.Stderrs[m.idx]
	}
	if m.idx < len(m.Errors) {
		err = m.Errors[m.idx]
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

// UI is a packersdk.Ui that discards everything.
type UI struct{}

var _ packersdk.Ui = &UI{}

func (u *UI) Say(message string)                                      {}
func (u *UI) Sayf(format string, args ...interface{})                 {}
func (u *UI) Message(message string)                                  {}
func (u *UI) Error(message string)                                    {}
func (u *UI) Errorf(format string, args ...interface{})               {}
func (u *UI) Ask(query string) (string, error)                        { return "", nil }
func (u *UI) Askf(format string, args ...interface{}) (string, error) { return "", nil }
func (u *UI) Machine(name string, args ...string)                     {}
func (u *UI) TrackProgress(src string, currentSize, totalSize int64, stream io.ReadCloser) io.ReadCloser {
	return stream
}

// BoolPtr returns a pointer to b, for *bool config fields.
func BoolPtr(b bool) *bool {
	return &b
}
