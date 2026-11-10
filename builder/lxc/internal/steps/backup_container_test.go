package steps

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/testutil"
)

func TestStepBackupContainer(t *testing.T) {
	cfg := &config.Config{
		BackupDir: "/var/lib/vz/template/cache",
	}
	ui := &testutil.UI{}
	// Sequence: vzdump success, ls success, mkdir success, mv success
	comm := &testutil.MockRunner{
		Outputs: []string{"", "/tmp/vzdump-lxc-100-2026_0503.tar.gz", "", ""},
		Errors:  []error{nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &BackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	if _, ok := state.GetOk(StateBackupPath); !ok {
		t.Errorf("Expected backup_path in state")
	}
}

// Each algorithm must reach vzdump AND drive the dump-file lookup and the
// artifact path, or the backup would succeed on the host and then be reported
// as missing because the step globbed for the wrong extension.
func TestStepBackupContainer_Compression(t *testing.T) {
	tests := []struct {
		name        string
		compression string
		wantVzdump  string
		wantLs      string
		wantPath    string
	}{
		{
			name:        "gzip",
			compression: "gzip",
			wantVzdump:  "vzdump 100 --compress gzip --dumpdir /tmp",
			wantLs:      "ls /tmp/vzdump-lxc-100-*.tar.gz 2>/dev/null | head -1",
			wantPath:    "/var/lib/vz/template/cache/lxc-template-100.tar.gz",
		},
		{
			name:        "zstd",
			compression: "zstd",
			wantVzdump:  "vzdump 100 --compress zstd --dumpdir /tmp",
			wantLs:      "ls /tmp/vzdump-lxc-100-*.tar.zst 2>/dev/null | head -1",
			wantPath:    "/var/lib/vz/template/cache/lxc-template-100.tar.zst",
		},
		{
			name:        "lzo",
			compression: "lzo",
			wantVzdump:  "vzdump 100 --compress lzo --dumpdir /tmp",
			wantLs:      "ls /tmp/vzdump-lxc-100-*.tar.lzo 2>/dev/null | head -1",
			wantPath:    "/var/lib/vz/template/cache/lxc-template-100.tar.lzo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				BackupDir:         "/var/lib/vz/template/cache",
				BackupCompression: tt.compression,
			}
			ui := &testutil.UI{}
			// Sequence: vzdump, ls, mkdir, mv
			comm := &testutil.MockRunner{
				Outputs: []string{"", "/tmp/vzdump-lxc-100-2026_0503.dump", "", ""},
				Errors:  []error{nil, nil, nil, nil},
			}

			state := new(multistep.BasicStateBag)
			state.Put(StateConfig, cfg)
			state.Put(StateUI, ui)
			state.Put(StateProxmoxComm, comm)
			state.Put(StateCTID, "100")

			step := &BackupContainer{}
			if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
				t.Fatalf("Expected ActionContinue, got %v", action)
			}

			if comm.Calls[0] != tt.wantVzdump {
				t.Errorf("Expected vzdump command %q, got %q", tt.wantVzdump, comm.Calls[0])
			}
			if comm.Calls[1] != tt.wantLs {
				t.Errorf("Expected lookup command %q, got %q", tt.wantLs, comm.Calls[1])
			}
			got, _ := state.Get(StateBackupPath).(string)
			if got != tt.wantPath {
				t.Errorf("Expected backup_path %q, got %q", tt.wantPath, got)
			}
		})
	}
}

// pigz is a parallel gzip; vzdump ignores --pigz for other algorithms, so the
// step must not probe for it or pass it when compression is not gzip.
func TestStepBackupContainer_NonGzipIgnoresPigz(t *testing.T) {
	cfg := &config.Config{
		BackupDir:         "/var/lib/vz/template/cache",
		BackupCompression: "zstd",
		BackupPigz:        4,
	}
	ui := &testutil.UI{}
	// Sequence: vzdump, ls, mkdir, mv -- deliberately NO `command -v pigz`.
	comm := &testutil.MockRunner{
		Outputs: []string{"", "/tmp/vzdump-lxc-100-2026_0503.tar.zst", "", ""},
		Errors:  []error{nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &BackupContainer{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("Expected ActionContinue, got %v", action)
	}

	for _, call := range comm.Calls {
		if strings.Contains(call, "pigz") {
			t.Errorf("Expected no pigz handling for zstd, got command %q", call)
		}
	}
	expected := "vzdump 100 --compress zstd --dumpdir /tmp"
	if comm.Calls[0] != expected {
		t.Errorf("Expected vzdump command %q, got %q", expected, comm.Calls[0])
	}
}

func TestStepBackupContainer_WithPigz(t *testing.T) {
	cfg := &config.Config{
		BackupDir:         "/var/lib/vz/template/cache",
		BackupCompression: "gzip",
		BackupPigz:        4,
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{
		// Sequence: command -v pigz (found), vzdump, ls, mkdir, mv
		Outputs: []string{"", "", "/tmp/vzdump-lxc-100-2026_0503.tar.gz", "", ""},
		Errors:  []error{nil, nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &BackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	if len(comm.Calls) < 2 {
		t.Fatalf("Expected at least 2 commands, got %d", len(comm.Calls))
	}
	if comm.Calls[0] != "command -v pigz" {
		t.Errorf("Expected first command to check for pigz, got %q", comm.Calls[0])
	}
	expectedVzdumpCmd := "vzdump 100 --compress gzip --dumpdir /tmp --pigz 4"
	if comm.Calls[1] != expectedVzdumpCmd {
		t.Errorf("Expected vzdump command %q, got %q", expectedVzdumpCmd, comm.Calls[1])
	}
}

func TestStepBackupContainer_PigzMissing_FallsBackToGzip(t *testing.T) {
	cfg := &config.Config{
		BackupDir:         "/var/lib/vz/template/cache",
		BackupCompression: "gzip",
		BackupPigz:        4,
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{
		// Sequence: command -v pigz (NOT found), vzdump (plain gzip), ls, mkdir, mv
		Outputs: []string{"", "", "/tmp/vzdump-lxc-100-2026_0503.tar.gz", "", ""},
		Errors:  []error{&someError{msg: "pigz: not found"}, nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &BackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	if len(comm.Calls) < 2 {
		t.Fatalf("Expected at least 2 commands, got %d", len(comm.Calls))
	}
	expectedVzdumpCmd := "vzdump 100 --compress gzip --dumpdir /tmp"
	if comm.Calls[1] != expectedVzdumpCmd {
		t.Errorf("Expected fallback to plain gzip %q, got %q", expectedVzdumpCmd, comm.Calls[1])
	}
}

func TestStepBackupContainer_PigzDisabled(t *testing.T) {
	cfg := &config.Config{
		BackupDir:         "/var/lib/vz/template/cache",
		BackupCompression: "gzip",
		BackupPigz:        -1, // explicitly disabled (resolved value after Config.Prepare)
	}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{
		Outputs: []string{"", "/tmp/vzdump-lxc-100-2026_0503.tar.gz", "", ""},
		Errors:  []error{nil, nil, nil, nil},
	}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &BackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionContinue {
		t.Errorf("Expected ActionContinue, got %v", action)
	}

	expectedVzdumpCmd := "vzdump 100 --compress gzip --dumpdir /tmp"
	if len(comm.Calls) == 0 || comm.Calls[0] != expectedVzdumpCmd {
		t.Errorf("Expected vzdump command %q, got %q", expectedVzdumpCmd, comm.Calls[0])
	}
}

func TestStepBackupContainer_Error(t *testing.T) {
	cfg := &config.Config{}
	ui := &testutil.UI{}
	comm := &testutil.MockRunner{Errors: []error{&someError{msg: "vzdump failed"}}}

	state := new(multistep.BasicStateBag)
	state.Put(StateConfig, cfg)
	state.Put(StateUI, ui)
	state.Put(StateProxmoxComm, comm)
	state.Put(StateCTID, "100")

	step := &BackupContainer{}
	action := step.Run(context.Background(), state)

	if action != multistep.ActionHalt {
		t.Errorf("Expected ActionHalt, got %v", action)
	}
}
