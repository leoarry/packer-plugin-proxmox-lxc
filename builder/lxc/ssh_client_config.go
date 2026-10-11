package lxc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	packerssh "github.com/hashicorp/packer-plugin-sdk/sdk-internals/communicator/ssh"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// sshAuth is a built SSH client configuration plus the resources that must be
// released once the build is done with it.
type sshAuth struct {
	Config *ssh.ClientConfig
	// closer is the SSH agent connection, nil when agent authentication is not
	// configured.
	closer io.Closer
}

// parsePrivateKeyFile loads a private key, decrypting it when a passphrase is
// configured. The error for an encrypted key without a passphrase names both
// options that can fix it.
func parsePrivateKeyFile(path, passphrase string) (ssh.Signer, error) {
	keyPath, err := expandUserPath(path)
	if err != nil {
		return nil, err
	}
	keyData, err := os.ReadFile(keyPath) // #nosec G304 -- path is the operator-configured ssh_private_key_file, not untrusted input
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key: %w", err)
	}

	var signer ssh.Signer
	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(keyData)
	}
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return nil, fmt.Errorf("SSH key %s is passphrase protected: set ssh_key_passphrase, "+
				"or load the key into an agent and set ssh_agent_auth = true", keyPath)
		}
		return nil, fmt.Errorf("failed to parse SSH key %s: %w", keyPath, err)
	}
	return signer, nil
}

// buildSSHClientConfig builds the SSH client configuration: the authentication
// methods in the order ssh offers them (agent, then key, then password) and the
// host key verification callback.
func buildSSHClientConfig(ctx context.Context, cfg *Config, ui packersdk.Ui) (*sshAuth, error) {
	result := &sshAuth{}
	auth := make([]ssh.AuthMethod, 0, 3)
	var agentSigners func() ([]ssh.Signer, error)
	var fileSigners []ssh.Signer

	if cfg.SSHAgentAuth {
		agentConn, err := packerssh.GetSSHAgentConnection()
		if err != nil {
			return nil, fmt.Errorf("ssh_agent_auth: %w", err)
		}
		// Close the agent connection if configuration fails before ownership
		// can be passed to the connect step.
		defer func() {
			if result.Config == nil {
				_ = agentConn.Close()
				result.closer = nil
			}
		}()
		result.closer = agentConn
		agentSigners = agent.NewClient(agentConn).Signers
	}

	if cfg.SSHKeyPath != "" {
		signer, err := parsePrivateKeyFile(cfg.SSHKeyPath, cfg.SSHKeyPassphrase)
		if err != nil {
			return nil, err
		}
		fileSigners = append(fileSigners, signer)
	}

	if agentSigners != nil {
		// SSH tries each authentication method only once. Offer agent and file
		// keys together so rejecting the agent keys cannot skip the file key.
		auth = append(auth, ssh.PublicKeysCallback(func() ([]ssh.Signer, error) {
			signers, err := agentSigners()
			if err != nil && len(fileSigners) == 0 {
				return nil, err
			}
			return append(signers, fileSigners...), nil
		}))
	} else if len(fileSigners) > 0 {
		auth = append(auth, ssh.PublicKeys(fileSigners...))
	}

	if cfg.SSHPassword != "" {
		auth = append(auth,
			ssh.Password(cfg.SSHPassword),
			ssh.KeyboardInteractive(packerssh.PasswordKeyboardInteractive(cfg.SSHPassword)))
	}

	if len(auth) == 0 {
		return nil, fmt.Errorf("no SSH authentication method configured")
	}

	hostKey, err := hostKeyCallback(ctx, cfg, ui)
	if err != nil {
		return nil, err
	}

	var hostKeyAlgorithms []string
	if cfg.SSHHostKeyCheck != HostKeyCheckOff {
		file, err := knownHostsPath(cfg)
		if err != nil {
			return nil, err
		}
		hostKeyAlgorithms, err = knownHostKeyAlgorithms(file, cfg.sshAddress())
		if err != nil {
			return nil, err
		}
	}
	result.Config = &ssh.ClientConfig{
		User:              cfg.SSHUser,
		Auth:              auth,
		HostKeyCallback:   hostKey,
		HostKeyAlgorithms: hostKeyAlgorithms,
	}
	return result, nil
}
