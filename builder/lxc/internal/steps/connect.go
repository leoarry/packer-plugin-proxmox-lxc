package steps

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

type Connect struct{}

func (s *Connect) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get(StateUI).(packersdk.Ui)
	cfg := state.Get(StateConfig).(*config.Config)

	ui.Say("Connecting to Proxmox host...")

	sshConfig := &ssh.ClientConfig{
		User:            cfg.SSHUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	if cfg.SSHPassword != "" {
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(cfg.SSHPassword)}
	} else if cfg.SSHKeyPath != "" {
		keyData, err := os.ReadFile(cfg.SSHKeyPath)
		if err != nil {
			state.Put(StateError, fmt.Errorf("failed to read SSH key: %w", err))
			return multistep.ActionHalt
		}
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			state.Put(StateError, fmt.Errorf("failed to parse SSH key: %w", err))
			return multistep.ActionHalt
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

	addr := fmt.Sprintf("%s:%d", cfg.SSHHost, cfg.SSHPort)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to connect to Proxmox host: %v", err))
		state.Put(StateError, fmt.Errorf("ssh connection failed: %w", err))
		return multistep.ActionHalt
	}

	comm := remote.NewSSHCommunicator(client)
	state.Put(StateCommunicator, comm)
	state.Put(StateSSHClient, client)
	ui.Say("Connected to Proxmox host")
	return multistep.ActionContinue
}

func (s *Connect) Cleanup(state multistep.StateBag) {
	if client, ok := state.GetOk(StateSSHClient); ok {
		if c, ok := client.(*ssh.Client); ok {
			_ = c.Close()
		}
	}
}
