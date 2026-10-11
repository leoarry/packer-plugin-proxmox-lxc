package lxc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

type stepConnect struct{}

func (s *stepConnect) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)

	ui.Say("Connecting to Proxmox host...")

	timeout, err := time.ParseDuration(config.SSHTimeout)
	if err != nil {
		state.Put("error", fmt.Errorf("invalid ssh_timeout: %w", err))
		return multistep.ActionHalt
	}

	address := config.sshAddress()
	deadline := time.Now().Add(timeout)

	var client *ssh.Client
	for attempt := 1; ; attempt++ {
		if ctxErr := ctx.Err(); ctxErr != nil {
			state.Put("error", fmt.Errorf("ssh connection to %s cancelled: %w", address, ctxErr))
			return multistep.ActionHalt
		}

		// Bind configuration callbacks and dialing to the same attempt budget.
		attemptCtx, cancel := sshConnectContext(ctx, deadline)
		auth, err := buildSSHClientConfig(attemptCtx, config, ui)
		if err != nil {
			cancel()
			state.Put("error", err)
			return multistep.ActionHalt
		}
		if prev, ok := state.GetOk("ssh_agent_conn"); ok {
			if c, ok := prev.(io.Closer); ok {
				_ = c.Close()
			}
		}
		if auth.closer != nil {
			state.Put("ssh_agent_conn", auth.closer)
		}

		var lastErr error
		client, lastErr = dialSSH(attemptCtx, address, auth.Config, deadline)
		cancel()
		if lastErr == nil {
			break
		}

		if attempt >= config.SSHHandshakeAttempts || time.Now().Add(sshRetryInterval).After(deadline) {
			ui.Error(fmt.Sprintf("Failed to connect to Proxmox host: %v", lastErr))
			state.Put("error", fmt.Errorf("ssh connection to %s failed after %d attempt(s): %w",
				address, attempt, lastErr))
			return multistep.ActionHalt
		}

		ui.Say(fmt.Sprintf("SSH to %s not ready (attempt %d/%d): %v; retrying in %s...",
			address, attempt, config.SSHHandshakeAttempts, lastErr, sshRetryInterval))

		select {
		case <-ctx.Done():
			state.Put("error", fmt.Errorf("ssh connection to %s cancelled: %w", address, ctx.Err()))
			return multistep.ActionHalt
		case <-time.After(sshRetryInterval):
		}
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
			_ = c.Close()
		}
	}
	if agentConn, ok := state.GetOk("ssh_agent_conn"); ok {
		if c, ok := agentConn.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}
}
