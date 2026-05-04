package lxc

import (
	"context"
	"fmt"
	"os"
	"time"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"golang.org/x/crypto/ssh"
)

type stepConnect struct{}

func (s *stepConnect) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)

	ui.Say("Connecting to Proxmox host...")

	sshConfig := &ssh.ClientConfig{
		User:            config.SSHUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	if config.SSHPassword != "" {
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(config.SSHPassword)}
	} else if config.SSHKeyPath != "" {
		keyData, err := os.ReadFile(config.SSHKeyPath)
		if err != nil {
			state.Put("error", fmt.Errorf("failed to read SSH key: %w", err))
			return multistep.ActionHalt
		}
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			state.Put("error", fmt.Errorf("failed to parse SSH key: %w", err))
			return multistep.ActionHalt
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

	addr := fmt.Sprintf("%s:%d", config.SSHHost, config.SSHPort)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		ui.Error(fmt.Sprintf("Failed to connect to Proxmox host: %v", err))
		state.Put("error", fmt.Errorf("ssh connection failed: %w", err))
		return multistep.ActionHalt
	}

	comm := &sshCommunicator{client: &sshClientWrapper{client: client}}
	state.Put("communicator", comm)
	state.Put("ssh_client", client)
	ui.Say("Connected to Proxmox host")
	return multistep.ActionContinue
}

func (s *stepConnect) Cleanup(state multistep.StateBag) {
	if client, ok := state.GetOk("ssh_client"); ok {
		if c, ok := client.(*ssh.Client); ok {
			c.Close()
		}
	}
}
