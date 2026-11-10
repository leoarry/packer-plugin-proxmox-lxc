package steps

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc/internal/config"
)

func newSigner(t *testing.T, kind string) ssh.Signer {
	t.Helper()
	var key any
	var err error
	switch kind {
	case "ed25519":
		_, key, err = ed25519.GenerateKey(rand.Reader)
	case "ecdsa":
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "rsa":
		key, err = rsa.GenerateKey(rand.Reader, 2048)
	}
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func writeKnownHosts(t *testing.T, addr string, keys ...ssh.PublicKey) string {
	t.Helper()
	var lines []string
	for _, k := range keys {
		lines = append(lines, knownhosts.Line([]string{knownhosts.Normalize(addr)}, k))
	}
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// startSSHServer runs an SSH server on localhost that accepts any password,
// presenting the given host keys. It returns host and port.
func startSSHServer(t *testing.T, hostKeys ...ssh.Signer) (string, int) {
	t.Helper()
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil },
	}
	for _, k := range hostKeys {
		cfg.AddHostKey(k)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				sc, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer func() { _ = sc.Close() }()
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					_ = ch.Reject(ssh.Prohibited, "test server")
				}
			}()
		}
	}()
	tcp := ln.Addr().(*net.TCPAddr)
	return tcp.IP.String(), tcp.Port
}

func testConnectConfig(host string, port int) *config.Config {
	return &config.Config{
		SSHHost:     host,
		SSHPort:     port,
		SSHUser:     "root",
		SSHPassword: "secret",
		SSHTimeout:  "5s",
	}
}

func dial(t *testing.T, c *config.Config) error {
	t.Helper()
	sshConfig, err := sshClientConfig(c)
	if err != nil {
		return err
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(c.SSHHost, strconv.Itoa(c.SSHPort)), sshConfig)
	if err == nil {
		_ = client.Close()
	}
	return err
}

func TestSSHClientConfig_UsesSSHTimeout(t *testing.T) {
	c := testConnectConfig("127.0.0.1", 22)
	c.SSHTimeout = "2m"
	sshConfig, err := sshClientConfig(c)
	if err != nil {
		t.Fatal(err)
	}
	if sshConfig.Timeout != 2*time.Minute {
		t.Errorf("Timeout = %s, want 2m", sshConfig.Timeout)
	}
}

func TestSSHClientConfig_MissingKnownHostsFile(t *testing.T) {
	c := testConnectConfig("127.0.0.1", 22)
	c.SSHKnownHostsFile = filepath.Join(t.TempDir(), "nope")
	if _, err := sshClientConfig(c); err == nil {
		t.Fatal("expected error for missing ssh_known_hosts_file")
	}
}

func TestSSHConnect_HostKeyVerification(t *testing.T) {
	ed := newSigner(t, "ed25519")
	ec := newSigner(t, "ecdsa")
	host, port := startSSHServer(t, ec, ed) // server has both keys
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	other := newSigner(t, "ed25519")

	tests := []struct {
		name    string
		known   []ssh.PublicKey // nil = ssh_known_hosts_file unset
		wantErr bool
	}{
		{name: "not verified when unset"},
		{name: "both keys recorded", known: []ssh.PublicKey{ed.PublicKey(), ec.PublicKey()}},
		// The server has two keys but only one is recorded: the client must
		// ask for that key type instead of failing with a false "key mismatch".
		{name: "only ed25519 recorded", known: []ssh.PublicKey{ed.PublicKey()}},
		{name: "only ecdsa recorded", known: []ssh.PublicKey{ec.PublicKey()}},
		{name: "mismatched key", known: []ssh.PublicKey{other.PublicKey()}, wantErr: true},
		{name: "host not in file", known: []ssh.PublicKey{}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testConnectConfig(host, port)
			if tt.known != nil {
				c.SSHKnownHostsFile = writeKnownHosts(t, addr, tt.known...)
				if len(tt.known) == 0 {
					c.SSHKnownHostsFile = writeKnownHosts(t, "other.example:22", other.PublicKey())
				}
			}
			err := dial(t, c)
			if (err != nil) != tt.wantErr {
				t.Errorf("dial error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKnownHostKeyAlgorithms(t *testing.T) {
	addr := "pve.example:22"
	rsaKey := newSigner(t, "rsa").PublicKey()
	edKey := newSigner(t, "ed25519").PublicKey()

	callback, err := knownhosts.New(writeKnownHosts(t, addr, rsaKey, edKey))
	if err != nil {
		t.Fatal(err)
	}
	got := knownHostKeyAlgorithms(callback, addr)
	for _, want := range []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA, ssh.KeyAlgoED25519} {
		if !slices.Contains(got, want) {
			t.Errorf("algorithms %v missing %s", got, want)
		}
	}
	if got := knownHostKeyAlgorithms(callback, "unknown.example:22"); got != nil {
		t.Errorf("unknown host: got %v, want nil", got)
	}
}

func TestSSHClientConfig_KeyPassphrase(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("hunter2"))
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		passphrase string
		wantErr    bool
	}{
		{"hunter2", false},
		{"wrong", true},
		{"", true},
	} {
		c := testConnectConfig("127.0.0.1", 22)
		c.SSHPassword = ""
		c.SSHKeyPath = keyPath
		c.SSHKeyPassphrase = tt.passphrase
		if _, err := sshClientConfig(c); (err != nil) != tt.wantErr {
			t.Errorf("passphrase %q: err = %v, wantErr %v", tt.passphrase, err, tt.wantErr)
		}
	}
}
