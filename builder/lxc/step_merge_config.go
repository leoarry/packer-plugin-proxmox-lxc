package lxc

import (
	"context"
	"fmt"
	"strings"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

type stepMergeConfig struct{}

func (s *stepMergeConfig) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	ctid := state.Get("ctid").(string)

	if config.LXCConfig == "" {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packersdk.Ui)
	comm := state.Get("communicator").(CommandRunner)

	ui.Say("Merging custom LXC config...")

	escaped := strings.ReplaceAll(config.LXCConfig, "'", "'\"'\"'")
	cmd := fmt.Sprintf("echo '%s' >> /etc/pve/lxc/%s.conf", escaped, ctid)

	_, err := comm.RunCommand(ctx, cmd)
	if err != nil {
		state.Put("error", fmt.Errorf("failed to merge LXC config: %w", err))
		return multistep.ActionHalt
	}

	ui.Say("LXC config merged")
	return multistep.ActionContinue
}

func (s *stepMergeConfig) Cleanup(state multistep.StateBag) {}
