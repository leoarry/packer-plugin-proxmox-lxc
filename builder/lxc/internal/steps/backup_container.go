package steps

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

type BackupContainer struct{}

func (s *BackupContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get(StateUI).(packersdk.Ui)
	cfg := state.Get(StateConfig).(*config.Config)
	ctid := state.Get(StateCTID).(string)
	comm := state.Get(StateProxmoxComm).(remote.CommandRunner)

	backupName := cfg.BackupName
	if backupName == "" {
		backupName = fmt.Sprintf("lxc-template-%s", ctid)
	}
	state.Put(StateBackupName, backupName)

	ui.Say(fmt.Sprintf("Backing up container %s...", ctid))

	backupExt := config.BackupCompressionExt[cfg.BackupCompression]

	vzdumpCmd := fmt.Sprintf("vzdump %s --compress %s --dumpdir /tmp", ctid, cfg.BackupCompression)
	// BackupPigz is -1 when the user explicitly disabled pigz; omitting
	// --pigz falls back to plain gzip, same as passing --pigz 0. pigz is a
	// parallel gzip, so vzdump ignores it for every other algorithm.
	if cfg.BackupCompression == "gzip" && cfg.BackupPigz > 0 {
		if pigzAvailable(ctx, comm) {
			vzdumpCmd += fmt.Sprintf(" --pigz %d", cfg.BackupPigz)
		} else {
			ui.Say("pigz not found on Proxmox host, falling back to plain gzip for vzdump compression")
		}
	}

	var vzdumpOut, vzdumpErr bytes.Buffer
	err := comm.RunCommand(ctx, vzdumpCmd, &vzdumpOut, &vzdumpErr)
	if err != nil {
		state.Put(StateError, fmt.Errorf("vzdump failed: %w\nstdout: %s\nstderr: %s",
			err, strings.TrimSpace(vzdumpOut.String()), strings.TrimSpace(vzdumpErr.String())))
		return multistep.ActionHalt
	}

	var stdout bytes.Buffer
	err = comm.RunCommand(ctx, fmt.Sprintf("ls /tmp/vzdump-lxc-%s-*.%s 2>/dev/null | head -1", ctid, backupExt), &stdout, nil)
	if err != nil {
		state.Put(StateError, fmt.Errorf("backup file not found: %w", err))
		return multistep.ActionHalt
	}

	backupFile := strings.TrimSpace(stdout.String())
	if backupFile == "" {
		state.Put(StateError, fmt.Errorf("backup file not found in /tmp"))
		return multistep.ActionHalt
	}

	err = comm.RunCommand(ctx, fmt.Sprintf("mkdir -p %s", cfg.BackupDir), nil, nil)
	if err != nil {
		state.Put(StateError, fmt.Errorf("failed to create backup dir: %w", err))
		return multistep.ActionHalt
	}

	targetPath := fmt.Sprintf("%s/%s.%s", cfg.BackupDir, backupName, backupExt)
	err = comm.RunCommand(ctx, fmt.Sprintf("mv %s %s", backupFile, targetPath), nil, nil)
	if err != nil {
		state.Put(StateError, fmt.Errorf("failed to move backup: %w", err))
		return multistep.ActionHalt
	}

	state.Put(StateBackupPath, targetPath)
	ui.Say(fmt.Sprintf("Backup saved as: %s", targetPath))
	return multistep.ActionContinue
}

func (s *BackupContainer) Cleanup(state multistep.StateBag) {}

// pigzAvailable reports whether the pigz binary is present on the Proxmox
// host. Checked before requesting --pigz so a host without it installed
// falls back to plain gzip instead of failing the whole backup.
func pigzAvailable(ctx context.Context, comm remote.CommandRunner) bool {
	return comm.RunCommand(ctx, "command -v pigz", nil, nil) == nil
}
