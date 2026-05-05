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

	err := comm.RunCommand(ctx, fmt.Sprintf("vzdump %s --compress gzip --dumpdir /tmp", ctid), nil, nil)
	if err != nil {
		state.Put("error", fmt.Errorf("vzdump failed: %w", err))
		return multistep.ActionHalt
	}

	var stdout bytes.Buffer
	err = comm.RunCommand(ctx, fmt.Sprintf("ls /tmp/vzdump-lxc-%s-*.tar.gz 2>/dev/null | head -1", ctid), &stdout, nil)
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

	targetPath := fmt.Sprintf("%s/%s.tar.gz", config.BackupDir, backupName)
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
