package lxc

import (
	"fmt"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// Artifact represents the LXC template backup created by the builder.
type Artifact struct {
	BackupPath string
	StateData  map[string]interface{}
}

// BuilderId returns the builder identifier.
func (a *Artifact) BuilderId() string {
	return "proxmox-lxc.builder"
}

// Files returns the list of files that make up this artifact.
func (a *Artifact) Files() []string {
	return []string{a.BackupPath}
}

// Id returns the artifact ID (the backup path).
func (a *Artifact) Id() string {
	return a.BackupPath
}

// String returns a human-readable description of the artifact.
func (a *Artifact) String() string {
	return fmt.Sprintf("LXC template created at: %s", a.BackupPath)
}

// State returns state data associated with this artifact.
func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

// Destroy removes the artifact (optional cleanup).
func (a *Artifact) Destroy() error {
	// Optionally remove the backup file
	// Implementation can be added if needed
	return nil
}

// Ensure Artifact implements packersdk.Artifact.
var _ packersdk.Artifact = &Artifact{}
