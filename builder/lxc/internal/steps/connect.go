package steps

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/remote"
)

type Connect struct{}

func (s *Connect) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get(StateUI).(packersdk.Ui)
	cfg := state.Get(StateConfig).(*config.Config)

	ui.Say("Connecting to Proxmox host...")

	sshConfig, err := sshClientConfig(cfg)
	if err != nil {
		state.Put(StateError, err)
		return multistep.ActionHalt
	}

	addr := net.JoinHostPort(cfg.SSHHost, strconv.Itoa(cfg.SSHPort))
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

// sshClientConfig builds the SSH client config for the Proxmox host from the
// builder config.
func sshClientConfig(cfg *config.Config) (*ssh.ClientConfig, error) {
	timeout, err := time.ParseDuration(cfg.SSHTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid ssh_timeout: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.SSHUser,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}

	if cfg.SSHKnownHostsFile != "" {
		callback, err := knownhosts.New(cfg.SSHKnownHostsFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read ssh_known_hosts_file: %w", err)
		}
		addr := net.JoinHostPort(cfg.SSHHost, strconv.Itoa(cfg.SSHPort))
		sshConfig.HostKeyCallback = callback
		sshConfig.HostKeyAlgorithms = knownHostKeyAlgorithms(callback, addr)
	}

	if cfg.SSHPassword != "" {
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(cfg.SSHPassword)}
	} else if cfg.SSHKeyPath != "" {
		keyData, err := os.ReadFile(cfg.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key: %w", err)
		}
		var signer ssh.Signer
		if cfg.SSHKeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(cfg.SSHKeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

	return sshConfig, nil
}

// knownHostKeyAlgorithms returns the host key algorithms recorded for addr, so
// the server is asked for a key type we can actually verify. Without this, a
// known_hosts file holding only e.g. an ed25519 key fails with "key mismatch"
// when the server happens to offer ecdsa first. Returns nil (SSH defaults) if
// the host is not in the file, which then fails verification as unknown.
func knownHostKeyAlgorithms(callback ssh.HostKeyCallback, addr string) []string {
	// Look the host up with a random key that can't match: the resulting
	// KeyError lists every key recorded for it.
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil
	}
	probe, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil
	}
	var keyErr *knownhosts.KeyError
	if !errors.As(callback(addr, &net.TCPAddr{}, probe), &keyErr) {
		return nil
	}

	var algos []string
	for _, known := range keyErr.Want {
		if known.Key.Type() == ssh.KeyAlgoRSA {
			// An ssh-rsa key also verifies the SHA-2 signature algorithms.
			algos = append(algos, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256)
		}
		algos = append(algos, known.Key.Type())
	}
	return algos
}
