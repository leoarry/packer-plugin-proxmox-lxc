package lxc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

const testKeyPassphrase = "correct horse battery staple"

// writeKey generates an ed25519 private key, optionally encrypted, and returns
// its path.
func writeKey(t *testing.T, passphrase string) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	var block *pem.Block
	if passphrase == "" {
		block, err = ssh.MarshalPrivateKey(priv, "")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte(passphrase))
	}
	if err != nil {
		t.Fatalf("marshalling key: %v", err)
	}

	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBuildSSHClientConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		config  func(t *testing.T) *Config
		wantErr string
	}{
		{
			name:    "no authentication method",
			config:  func(*testing.T) *Config { return &Config{SSHHostKeyCheck: HostKeyCheckOff} },
			wantErr: "no SSH authentication method configured",
		},
		{
			name: "encrypted key without passphrase",
			config: func(t *testing.T) *Config {
				return &Config{SSHKeyPath: writeKey(t, testKeyPassphrase), SSHHostKeyCheck: HostKeyCheckOff}
			},
			wantErr: "is passphrase protected",
		},
		{
			name: "wrong passphrase",
			config: func(t *testing.T) *Config {
				return &Config{
					SSHKeyPath:       writeKey(t, testKeyPassphrase),
					SSHKeyPassphrase: "not the passphrase",
					SSHHostKeyCheck:  HostKeyCheckOff,
				}
			},
			wantErr: "failed to parse SSH key",
		},
		{
			name: "agent enabled without SSH_AUTH_SOCK",
			config: func(*testing.T) *Config {
				return &Config{SSHAgentAuth: true, SSHHostKeyCheck: HostKeyCheckOff}
			},
			wantErr: "SSH_AUTH_SOCK",
		},
		{
			name: "unknown host key policy",
			config: func(t *testing.T) *Config {
				return &Config{SSHPassword: "secret", SSHHostKeyCheck: "bogus"}
			},
			wantErr: "ssh_host_key_check must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SSH_AUTH_SOCK", "")
			_, err := buildSSHClientConfig(context.Background(), tt.config(t), &testUi{})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("buildSSHClientConfig() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestBuildSSHClientConfigKeyWithPassphrase(t *testing.T) {
	cfg := &Config{
		SSHKeyPath:       writeKey(t, testKeyPassphrase),
		SSHKeyPassphrase: testKeyPassphrase,
		SSHHostKeyCheck:  HostKeyCheckOff,
	}

	auth, err := buildSSHClientConfig(context.Background(), cfg, &testUi{})
	if err != nil {
		t.Fatalf("buildSSHClientConfig() error = %v", err)
	}
	if len(auth.Config.Auth) != 1 {
		t.Fatalf("len(Auth) = %d, want 1", len(auth.Config.Auth))
	}

	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	if err := fullAuthAgainstServer(t, srv, auth); err != nil {
		t.Fatalf("authenticating with the passphrase-protected key: %v", err)
	}
	if got := srv.AuthUsed(); got != "publickey" {
		t.Errorf("authentication method used = %q, want %q", got, "publickey")
	}
}

func TestBuildSSHClientConfigPasswordOffersBothMethods(t *testing.T) {
	auth, err := buildSSHClientConfig(context.Background(), &Config{
		SSHPassword:     "secret",
		SSHHostKeyCheck: HostKeyCheckOff,
	}, &testUi{})
	if err != nil {
		t.Fatalf("buildSSHClientConfig() error = %v", err)
	}
	if len(auth.Config.Auth) != 2 {
		t.Fatalf("len(Auth) = %d, want password and keyboard-interactive", len(auth.Config.Auth))
	}
	if auth.closer != nil {
		t.Error("no agent was configured, so closer must be nil")
	}
}

func TestBuildSSHClientConfigPrefersAgentOverPassword(t *testing.T) {
	keyPath := writeKey(t, "")
	startTestAgent(t, keyPath)

	cfg := &Config{
		SSHKeyPath:      keyPath,
		SSHPassword:     "secret",
		SSHAgentAuth:    true,
		SSHHostKeyCheck: HostKeyCheckOff,
	}
	auth, err := buildSSHClientConfig(context.Background(), cfg, &testUi{})
	if err != nil {
		t.Fatalf("buildSSHClientConfig() error = %v", err)
	}
	if auth.closer == nil {
		t.Fatal("expected the agent connection to be returned for closing")
	}
	defer func() { _ = auth.closer.Close() }()

	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	if err := fullAuthAgainstServer(t, srv, auth); err != nil {
		t.Fatalf("authenticating: %v", err)
	}
	if got := srv.AuthUsed(); got != "publickey" {
		t.Errorf("authentication method used = %q, want the agent key (%q)", got, "publickey")
	}
}

func TestBuildSSHClientConfigAgentAndKeyFile(t *testing.T) {
	for _, name := range []string{"agent accepted", "agent rejected", "agent disconnected"} {
		t.Run(name, func(t *testing.T) {
			agentPath := writeKey(t, "")
			keyPath := writeKey(t, "")
			startTestAgent(t, agentPath)
			auth, err := buildSSHClientConfig(context.Background(), &Config{
				SSHUser: "root", SSHAgentAuth: true, SSHKeyPath: keyPath,
				SSHHostKeyCheck: HostKeyCheckOff,
			}, &testUi{})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = auth.closer.Close() })

			acceptedPath := keyPath
			switch name {
			case "agent accepted":
				acceptedPath = agentPath
			case "agent disconnected":
				if err := auth.closer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			signer, err := parsePrivateKeyFile(acceptedPath, "")
			if err != nil {
				t.Fatal(err)
			}
			srv := startTestSSHServer(t, testSSHServerOptions{AcceptedPublicKey: signer.PublicKey()})
			if err := fullAuthAgainstServer(t, srv, auth); err != nil {
				t.Fatalf("authenticating with %s: %v", name, err)
			}
		})
	}
}

func countOpenFDs(t *testing.T) int {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("fd counting uses /proc/self/fd")
	}
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("cannot read /proc/self/fd: %v", err)
	}
	return len(entries)
}

func TestBuildSSHClientConfigDoesNotLeakAgentConnections(t *testing.T) {
	startTestAgent(t, writeKey(t, ""))
	cfg := &Config{
		SSHAgentAuth:    true,
		SSHKeyPath:      filepath.Join(t.TempDir(), "does-not-exist"),
		SSHHostKeyCheck: HostKeyCheckOff,
	}

	before := countOpenFDs(t)
	const attempts = 5
	for i := 0; i < attempts; i++ {
		if _, err := buildSSHClientConfig(context.Background(), cfg, &testUi{}); err == nil {
			t.Fatal("expected the build to fail on the missing key file")
		}
	}
	after := countOpenFDs(t)
	// The harness reaps its end of each connection asynchronously, so the
	// count is polled until it settles; only a persistent leak stays above
	// the margin.
	deadline := time.Now().Add(2 * time.Second)
	for after > before+2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		after = countOpenFDs(t)
	}

	// A small margin absorbs unrelated transient descriptors; a leaked agent
	// connection per failed build would show up as +attempts.
	if after > before+2 {
		t.Errorf("open fds grew from %d to %d across %d failed builds: the agent connection leaks",
			before, after, attempts)
	}
}
