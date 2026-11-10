package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

type MergeConfig struct{}

func (s *MergeConfig) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	cfg := state.Get(StateConfig).(*config.Config)
	ctid := state.Get(StateCTID).(string)

	if cfg.LXCConfig == "" {
		return multistep.ActionContinue
	}

	ui := state.Get(StateUI).(packersdk.Ui)
	comm := state.Get(StateCommunicator).(remote.CommandRunner)

	ui.Say("Merging custom LXC config...")

	escaped := strings.ReplaceAll(cfg.LXCConfig, "'", "'\"'\"'")
	cmd := fmt.Sprintf("echo '%s' >> /etc/pve/lxc/%s.conf", escaped, ctid)

	err := comm.RunCommand(ctx, cmd, nil, nil)
	if err != nil {
		state.Put(StateError, fmt.Errorf("failed to merge LXC config: %w", err))
		return multistep.ActionHalt
	}

	ui.Say("LXC config merged")
	return multistep.ActionContinue
}

func (s *MergeConfig) Cleanup(state multistep.StateBag) {}
