package lxc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func testConfigForServer(t *testing.T, srv *testSSHServer, policy string) *Config {
	t.Helper()
	host, portStr, err := net.SplitHostPort(srv.Address)
	if err != nil {
		t.Fatalf("splitting %q: %v", srv.Address, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("port %q: %v", portStr, err)
	}
	return &Config{
		SSHHost:           host,
		SSHPort:           port,
		SSHUser:           "root",
		SSHPassword:       "secret",
		SSHHostKeyCheck:   policy,
		SSHKnownHostsFile: filepath.Join(t.TempDir(), "known_hosts"),
		SSHTimeout:        "2s",
	}
}

// dialWith runs a real handshake using only the callback under test, so this
// task does not depend on the authentication code.
func dialWith(t *testing.T, srv *testSSHServer, cfg *Config) error {
	t.Helper()
	hostKey, err := hostKeyCallback(context.Background(), cfg, &testUi{})
	if err != nil {
		return err
	}
	client, err := dialSSH(context.Background(), cfg.sshAddress(), &ssh.ClientConfig{
		User:            cfg.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.Password(cfg.SSHPassword)},
		HostKeyCallback: hostKey,
	}, time.Now().Add(sshDialTimeout))
	if err != nil {
		return err
	}
	return client.Close()
}

func writeKnownHostEntry(t *testing.T, cfg *Config, hostKey ssh.PublicKey) {
	t.Helper()
	if err := appendKnownHostEntry(cfg.SSHKnownHostsFile, cfg.sshAddress(), hostKey); err != nil {
		t.Fatalf("appendKnownHostEntry: %v", err)
	}
}

func TestHostKeyStrictUnknownHostFails(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, HostKeyCheckStrict)

	err := dialWith(t, srv, cfg)
	if err == nil {
		t.Fatal("expected the connection to fail for an unknown host")
	}
	if !strings.Contains(err.Error(), "key is unknown") {
		t.Errorf("error = %v, want it to mention an unknown key", err)
	}
	if !strings.Contains(err.Error(), "ssh-keyscan") {
		t.Errorf("error = %v, want it to contain the ssh-keyscan remediation", err)
	}
}

func TestHostKeyStrictKnownHostConnects(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, HostKeyCheckStrict)
	writeKnownHostEntry(t, cfg, srv.HostKey)

	if err := dialWith(t, srv, cfg); err != nil {
		t.Fatalf("dialWith() error = %v, want nil", err)
	}
}

func TestHostKeyChangedKeyIsFatalInEveryVerifyingPolicy(t *testing.T) {
	for _, policy := range []string{HostKeyCheckStrict, HostKeyCheckAcceptNew} {
		t.Run(policy, func(t *testing.T) {
			srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
			cfg := testConfigForServer(t, srv, policy)

			other := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
			writeKnownHostEntry(t, cfg, other.HostKey)

			err := dialWith(t, srv, cfg)
			if err == nil {
				t.Fatal("expected the connection to fail after a host key change")
			}
			if !strings.Contains(err.Error(), "key mismatch") {
				t.Errorf("error = %v, want it to mention a key mismatch", err)
			}
		})
	}
}

func TestHostKeyAcceptNewRecordsUnknownHostOnce(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, HostKeyCheckAcceptNew)

	if err := dialWith(t, srv, cfg); err != nil {
		t.Fatalf("first dialWith() error = %v, want nil", err)
	}

	contents, err := os.ReadFile(cfg.SSHKnownHostsFile)
	if err != nil {
		t.Fatalf("reading known_hosts: %v", err)
	}
	want := knownhosts.Line([]string{knownhosts.Normalize(cfg.sshAddress())}, srv.HostKey)
	if !strings.Contains(string(contents), want) {
		t.Errorf("known_hosts = %q, want it to contain %q", contents, want)
	}

	before := len(contents)
	if err := dialWith(t, srv, cfg); err != nil {
		t.Fatalf("second dialWith() error = %v, want nil", err)
	}
	after, err := os.ReadFile(cfg.SSHKnownHostsFile)
	if err != nil {
		t.Fatalf("reading known_hosts: %v", err)
	}
	if len(after) != before {
		t.Errorf("known_hosts grew from %d to %d bytes: the key was recorded twice", before, len(after))
	}
}

func TestHostKeyOffConnectsDespiteMismatch(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, HostKeyCheckOff)

	other := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	writeKnownHostEntry(t, cfg, other.HostKey)

	if err := dialWith(t, srv, cfg); err != nil {
		t.Fatalf("dialWith() error = %v, want nil with ssh_host_key_check = \"off\"", err)
	}
}

func TestHostKeyCallbackRejectsUnknownPolicy(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, "bogus")

	if _, err := hostKeyCallback(context.Background(), cfg, &testUi{}); err == nil ||
		!strings.Contains(err.Error(), "ssh_host_key_check must be one of") {
		t.Fatalf("hostKeyCallback() error = %v, want an invalid policy error", err)
	}
}

func TestKnownHostsEntryUsesPortSyntax(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, HostKeyCheckAcceptNew)
	cfg.SSHPort = 2222

	if err := appendKnownHostEntry(cfg.SSHKnownHostsFile, cfg.sshAddress(), srv.HostKey); err != nil {
		t.Fatalf("appendKnownHostEntry: %v", err)
	}
	contents, err := os.ReadFile(cfg.SSHKnownHostsFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "[" + cfg.SSHHost + "]:2222 "
	if !strings.HasPrefix(string(contents), want) {
		t.Errorf("known_hosts = %q, want it to start with %q", contents, want)
	}
}

func TestHostKeyAcceptNewCreatesMissingFile(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, HostKeyCheckAcceptNew)
	cfg.SSHKnownHostsFile = filepath.Join(t.TempDir(), "nested", "dir", "known_hosts")

	if err := dialWith(t, srv, cfg); err != nil {
		t.Fatalf("dialWith() error = %v, want nil", err)
	}
	// Only existence is asserted: the exact 0644/0700 modes depend on the
	// process umask, and asserting them would make this test flaky.
	if _, err := os.Stat(cfg.SSHKnownHostsFile); err != nil {
		t.Fatalf("known_hosts was not created: %v", err)
	}
}

func TestHostKeyAcceptNewUnwritableFileIsActionable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	dir := t.TempDir()
	cfg := testConfigForServer(t, srv, HostKeyCheckAcceptNew)
	cfg.SSHKnownHostsFile = filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(cfg.SSHKnownHostsFile, nil, 0o444); err != nil { // #nosec G306 -- 0444 is the fixture: the known_hosts file must be unwritable to exercise the failure path
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil { // #nosec G302 -- 0500 is the fixture: the directory must be non-writable to exercise the failure path
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302 -- 0700 restores the standard private directory mode

	err := dialWith(t, srv, cfg)
	if err == nil {
		t.Fatal("expected the connection to fail when the known hosts file cannot be written")
	}
	for _, want := range []string{"ssh_known_hosts_file", HostKeyCheckStrict, HostKeyCheckOff} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %v, want it to mention %q", err, want)
		}
	}
}

func TestHostKeyStrictUnknownHostWithExistingFileCarriesRemediation(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, HostKeyCheckStrict)

	// The file exists but does not list this server: the wrap (not the
	// missing-file path) must produce the error.
	other := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	if err := appendKnownHostEntry(cfg.SSHKnownHostsFile, other.Address, other.HostKey); err != nil {
		t.Fatal(err)
	}

	err := dialWith(t, srv, cfg)
	if err == nil {
		t.Fatal("expected the connection to fail for an unlisted host")
	}
	if !strings.Contains(err.Error(), "knownhosts: key is unknown") {
		t.Errorf("error = %v, want the wrapped knownhosts error", err)
	}
	if !strings.Contains(err.Error(), "ssh-keyscan") {
		t.Errorf("error = %v, want the remediation command", err)
	}
	if !strings.Contains(err.Error(), cfg.SSHKnownHostsFile) {
		t.Errorf("error = %v, want it to name the known_hosts file", err)
	}
}

func TestHostKeyRemediationNamesTheExactCommand(t *testing.T) {
	got := hostKeyRemediation("192.168.1.100:2222", "/tmp/kh")
	if want := "ssh-keyscan -H -p 2222 192.168.1.100 >> /tmp/kh"; !strings.Contains(got, want) {
		t.Errorf("hostKeyRemediation() = %q, want it to contain %q", got, want)
	}

	got = hostKeyRemediation("192.168.1.100:22", "/tmp/kh")
	if want := "ssh-keyscan -H 192.168.1.100 >> /tmp/kh"; !strings.Contains(got, want) {
		t.Errorf("hostKeyRemediation() = %q, want it to contain %q", got, want)
	}
}
