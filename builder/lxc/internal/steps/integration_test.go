//go:build integration

package steps

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
)

// integrationConfig builds a config from the same PROXMOX_* variables as the
// builder integration tests (see builder/lxc/integration_test.go), and skips
// the test when they are not set. PROXMOX_KEY_PASSPHRASE is optional: set it
// when PROXMOX_KEY_PATH is an encrypted key.
func integrationConfig(t *testing.T) *config.Config {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	cfg := &config.Config{
		SSHHost:          os.Getenv("PROXMOX_HOST"),
		SSHPort:          22,
		SSHUser:          os.Getenv("PROXMOX_USER"),
		SSHPassword:      os.Getenv("PROXMOX_PASSWORD"),
		SSHKeyPath:       os.Getenv("PROXMOX_KEY_PATH"),
		SSHKeyPassphrase: os.Getenv("PROXMOX_KEY_PASSPHRASE"),
		SSHTimeout:       "30s",
	}
	if cfg.SSHHost == "" || cfg.SSHUser == "" {
		t.Skip("PROXMOX_HOST and PROXMOX_USER must be set")
	}
	if cfg.SSHPassword == "" && cfg.SSHKeyPath == "" {
		t.Skip("Either PROXMOX_PASSWORD or PROXMOX_KEY_PATH must be set")
	}
	if p := os.Getenv("PROXMOX_PORT"); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil {
			t.Fatalf("Invalid PROXMOX_PORT: %v", err)
		}
		cfg.SSHPort = port
	}
	return cfg
}

func integrationAddr(cfg *config.Config) string {
	return net.JoinHostPort(cfg.SSHHost, strconv.Itoa(cfg.SSHPort))
}

// connectWith dials the host the way the Connect step does.
func connectWith(cfg *config.Config) error {
	sshConfig, err := sshClientConfig(cfg)
	if err != nil {
		return err
	}
	client, err := ssh.Dial("tcp", integrationAddr(cfg), sshConfig)
	if err != nil {
		return err
	}
	return client.Close()
}

// hostKeys reads the host's public host keys over SSH, keyed by type.
func hostKeys(t *testing.T, cfg *config.Config) map[string]ssh.PublicKey {
	t.Helper()
	sshConfig, err := sshClientConfig(cfg) // no known_hosts file: unverified
	if err != nil {
		t.Fatal(err)
	}
	client, err := ssh.Dial("tcp", integrationAddr(cfg), sshConfig)
	if err != nil {
		t.Fatalf("connect to read host keys: %v", err)
	}
	defer func() { _ = client.Close() }()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close() }()
	out, err := session.Output("cat /etc/ssh/ssh_host_*_key.pub")
	if err != nil {
		t.Fatalf("read host keys: %v", err)
	}

	keys := map[string]ssh.PublicKey{}
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		key, _, _, _, err := ssh.ParseAuthorizedKey(line)
		if err != nil {
			t.Fatalf("parse host key %q: %v", line, err)
		}
		keys[key.Type()] = key
	}
	if len(keys) == 0 {
		t.Fatal("no host keys found in /etc/ssh")
	}
	return keys
}

func writeHostKnownHosts(t *testing.T, addr string, keys ...ssh.PublicKey) string {
	t.Helper()
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(knownhosts.Line([]string{knownhosts.Normalize(addr)}, k) + "\n")
	}
	path := t.TempDir() + "/known_hosts"
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestIntegration_SSHHostKeyVerification checks ssh_known_hosts_file against
// the real Proxmox sshd. It only opens SSH connections; nothing is created on
// the host.
func TestIntegration_SSHHostKeyVerification(t *testing.T) {
	cfg := integrationConfig(t)
	addr := integrationAddr(cfg)
	keys := hostKeys(t, cfg)

	var all []ssh.PublicKey
	for _, k := range keys {
		all = append(all, k)
	}

	t.Run("all host keys recorded", func(t *testing.T) {
		c := *cfg
		c.SSHKnownHostsFile = writeHostKnownHosts(t, addr, all...)
		if err := connectWith(&c); err != nil {
			t.Fatalf("connect: %v", err)
		}
	})

	// The server offers several key types; with only one recorded, the
	// client must ask for that type instead of failing with "key mismatch".
	for keyType, key := range keys {
		t.Run("only "+keyType+" recorded", func(t *testing.T) {
			c := *cfg
			c.SSHKnownHostsFile = writeHostKnownHosts(t, addr, key)
			if err := connectWith(&c); err != nil {
				t.Fatalf("connect: %v", err)
			}
		})
	}

	t.Run("wrong key recorded", func(t *testing.T) {
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		wrong, err := ssh.NewPublicKey(pub)
		if err != nil {
			t.Fatal(err)
		}
		c := *cfg
		c.SSHKnownHostsFile = writeHostKnownHosts(t, addr, wrong)
		err = connectWith(&c)
		if err == nil || !strings.Contains(err.Error(), "key mismatch") {
			t.Fatalf("expected a key mismatch error, got %v", err)
		}
	})

	t.Run("host not in file", func(t *testing.T) {
		c := *cfg
		c.SSHKnownHostsFile = writeHostKnownHosts(t, "not-this-host.invalid:22", all[0])
		err := connectWith(&c)
		if err == nil || !strings.Contains(err.Error(), "key is unknown") {
			t.Fatalf("expected an unknown key error, got %v", err)
		}
	})
}

// TestIntegration_SSHKeyPassphrase runs only when PROXMOX_KEY_PATH is an
// encrypted key and PROXMOX_KEY_PASSPHRASE is set. The right passphrase is
// already exercised by every other integration test; this checks a wrong one
// is rejected before connecting.
func TestIntegration_SSHKeyPassphrase(t *testing.T) {
	cfg := integrationConfig(t)
	if cfg.SSHKeyPassphrase == "" || cfg.SSHKeyPath == "" {
		t.Skip("PROXMOX_KEY_PATH and PROXMOX_KEY_PASSPHRASE must be set")
	}
	cfg.SSHPassword = "" // make sure the key is what authenticates

	if err := connectWith(cfg); err != nil {
		t.Fatalf("connect with passphrase: %v", err)
	}
	c := *cfg
	c.SSHKeyPassphrase = "wrong-" + cfg.SSHKeyPassphrase
	if err := connectWith(&c); err == nil {
		t.Fatal("expected an error with a wrong passphrase")
	}
}
