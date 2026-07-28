package lxc

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepBackupContainer struct{}

func (s *stepBackupContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	ctid := state.Get("ctid").(string)
	comm := state.Get("proxmox_comm").(CommandRunner)

	backupName := config.BackupName
	if backupName == "" {
		backupName = fmt.Sprintf("lxc-template-%s", ctid)
	}
	state.Put("backup_name", backupName)

	ui.Say(fmt.Sprintf("Backing up container %s...", ctid))

	compression, ext := resolveBackupCompression(config.BackupCompression)

	vzdumpCmd := fmt.Sprintf("vzdump %s --compress %s --dumpdir /tmp", ctid, compression)
	// BackupPigz is -1 when the user explicitly disabled pigz; omitting
	// --pigz falls back to plain gzip, same as passing --pigz 0. pigz is a
	// parallel gzip, so vzdump ignores it for every other algorithm.
	if compression == "gzip" && config.BackupPigz > 0 {
		if pigzAvailable(ctx, comm) {
			vzdumpCmd += fmt.Sprintf(" --pigz %d", config.BackupPigz)
		} else {
			ui.Say("pigz not found on Proxmox host, falling back to plain gzip for vzdump compression")
		}
	}

	var vzdumpOut, vzdumpErr bytes.Buffer
	err := comm.RunCommand(ctx, vzdumpCmd, &vzdumpOut, &vzdumpErr)
	if err != nil {
		state.Put("error", fmt.Errorf("vzdump failed: %w\nstdout: %s\nstderr: %s",
			err, strings.TrimSpace(vzdumpOut.String()), strings.TrimSpace(vzdumpErr.String())))
		return multistep.ActionHalt
	}

	var stdout bytes.Buffer
	err = comm.RunCommand(ctx, fmt.Sprintf("ls /tmp/vzdump-lxc-%s-*.%s 2>/dev/null | head -1", ctid, ext), &stdout, nil)
	if err != nil {
		state.Put("error", fmt.Errorf("backup file not found: %w", err))
		return multistep.ActionHalt
	}

	backupFile := strings.TrimSpace(stdout.String())
	if backupFile == "" {
		state.Put("error", fmt.Errorf("backup file not found in /tmp"))
		return multistep.ActionHalt
	}

	err = comm.RunCommand(ctx, fmt.Sprintf("mkdir -p %s", config.BackupDir), nil, nil)
	if err != nil {
		state.Put("error", fmt.Errorf("failed to create backup dir: %w", err))
		return multistep.ActionHalt
	}

	targetPath := fmt.Sprintf("%s/%s.%s", config.BackupDir, backupName, ext)
	err = comm.RunCommand(ctx, fmt.Sprintf("mv %s %s", backupFile, targetPath), nil, nil)
	if err != nil {
		state.Put("error", fmt.Errorf("failed to move backup: %w", err))
		return multistep.ActionHalt
	}

	state.Put("backup_path", targetPath)
	ui.Say(fmt.Sprintf("Backup saved as: %s", targetPath))
	return multistep.ActionContinue
}

func (s *stepBackupContainer) Cleanup(state multistep.StateBag) {}

// pigzAvailable reports whether the pigz binary is present on the Proxmox
// host. Checked before requesting --pigz so a host without it installed
// falls back to plain gzip instead of failing the whole backup.
func pigzAvailable(ctx context.Context, comm CommandRunner) bool {
	return comm.RunCommand(ctx, "command -v pigz", nil, nil) == nil
}
