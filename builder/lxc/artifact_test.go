package lxc

import (
	"testing"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

func TestArtifact_BuilderId(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		expected string
	}{
		{
			name: "basic artifact",
			artifact: &Artifact{
				BackupPath: "/var/lib/vz/template/cache/template-100.tar.gz",
				StateData:  map[string]interface{}{},
			},
			expected: "proxmox-lxc.builder",
		},
		{
			name:     "nil state data",
			artifact: &Artifact{BackupPath: "/tmp/test.tar.gz"},
			expected: "proxmox-lxc.builder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.BuilderId()
			if got != tt.expected {
				t.Errorf("BuilderId() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestArtifact_Files(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		expected []string
	}{
		{
			name: "single backup file",
			artifact: &Artifact{
				BackupPath: "/var/lib/vz/template/cache/template-100.tar.gz",
				StateData:  map[string]interface{}{},
			},
			expected: []string{"/var/lib/vz/template/cache/template-100.tar.gz"},
		},
		{
			name: "different path",
			artifact: &Artifact{
				BackupPath: "/tmp/my-template.tar.gz",
				StateData:  map[string]interface{}{},
			},
			expected: []string{"/tmp/my-template.tar.gz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.Files()
			if len(got) != len(tt.expected) {
				t.Fatalf("Files() length = %d, want %d", len(got), len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("Files()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestArtifact_Id(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		expected string
	}{
		{
			name: "basic id",
			artifact: &Artifact{
				BackupPath: "/var/lib/vz/template/cache/template-100.tar.gz",
			},
			expected: "/var/lib/vz/template/cache/template-100.tar.gz",
		},
		{
			name: "empty path",
			artifact: &Artifact{
				BackupPath: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.Id()
			if got != tt.expected {
				t.Errorf("Id() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestArtifact_String(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		expected string
	}{
		{
			name: "basic string",
			artifact: &Artifact{
				BackupPath: "/var/lib/vz/template/cache/template-100.tar.gz",
			},
			expected: "LXC template created at: /var/lib/vz/template/cache/template-100.tar.gz",
		},
		{
			name: "different path",
			artifact: &Artifact{
				BackupPath: "/tmp/test.tar.gz",
			},
			expected: "LXC template created at: /tmp/test.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.String()
			if got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestArtifact_State(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		key      string
		expected interface{}
	}{
		{
			name: "existing string key",
			artifact: &Artifact{
				StateData: map[string]interface{}{
					"backup_name": "template-100",
					"ctid":        "100",
				},
			},
			key:      "backup_name",
			expected: "template-100",
		},
		{
			name: "existing int key",
			artifact: &Artifact{
				StateData: map[string]interface{}{
					"backup_name": "template-100",
					"ctid":        "100",
				},
			},
			key:      "ctid",
			expected: "100",
		},
		{
			name: "non-existing key",
			artifact: &Artifact{
				StateData: map[string]interface{}{
					"backup_name": "template-100",
				},
			},
			key:      "non_existing",
			expected: nil,
		},
		{
			name: "nil state data",
			artifact: &Artifact{
				StateData: nil,
			},
			key:      "any_key",
			expected: nil,
		},
		{
			name: "empty state data",
			artifact: &Artifact{
				StateData: map[string]interface{}{},
			},
			key:      "any_key",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.artifact.State(tt.key)
			if got != tt.expected {
				t.Errorf("State(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

func TestArtifact_Destroy(t *testing.T) {
	tests := []struct {
		name     string
		artifact *Artifact
		wantErr  bool
	}{
		{
			name: "destroy basic artifact",
			artifact: &Artifact{
				BackupPath: "/var/lib/vz/template/cache/template-100.tar.gz",
			},
			wantErr: false,
		},
		{
			name: "destroy with state data",
			artifact: &Artifact{
				BackupPath: "/tmp/test.tar.gz",
				StateData: map[string]interface{}{
					"backup_name": "test",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.artifact.Destroy()
			if (err != nil) != tt.wantErr {
				t.Errorf("Destroy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestArtifact_Implements(t *testing.T) {
	var _ packersdk.Artifact = &Artifact{}
}
