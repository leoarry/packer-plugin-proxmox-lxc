package lxc

import (
	"fmt"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// Artifact represents the LXC template created by the builder, either a
// vzdump backup file (Method == "vzdump", the default) or a Proxmox CT
// template left in place on the Proxmox host (Method == "template").
type Artifact struct {
	Method     string
	BackupPath string
	CTID       string
	StateData  map[string]interface{}
}

// BuilderId returns the builder identifier.
func (a *Artifact) BuilderId() string {
	return "proxmox-lxc.builder"
}

// Files returns the list of files that make up this artifact.
// A CT template artifact lives on the Proxmox host, not as a local file,
// so it has no associated files.
func (a *Artifact) Files() []string {
	if a.Method == "template" {
		return nil
	}
	return []string{a.BackupPath}
}

// Id returns the artifact ID: the backup path, or the CTID for a CT template.
func (a *Artifact) Id() string {
	if a.Method == "template" {
		return a.CTID
	}
	return a.BackupPath
}

// String returns a human-readable description of the artifact.
func (a *Artifact) String() string {
	if a.Method == "template" {
		return fmt.Sprintf("LXC CT template created: CTID %s", a.CTID)
	}
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
