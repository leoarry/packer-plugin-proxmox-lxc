package lxc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/fips140"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func testHostSigner(t *testing.T, rsaKey bool) ssh.Signer {
	t.Helper()
	var key interface{}
	if rsaKey {
		var err error
		key, err = rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		_, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		key = private
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func testHostCertificate(t *testing.T, host, ca ssh.Signer) ssh.Signer {
	t.Helper()
	cert := &ssh.Certificate{
		Key: host.PublicKey(), CertType: ssh.HostCert,
		ValidPrincipals: []string{"127.0.0.1"},
		ValidAfter:      0,
		ValidBefore:     ssh.CertTimeInfinity,
	}
	if err := cert.SignCert(rand.Reader, ca); err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewCertSigner(cert, host)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func TestSSHConnectSelectsRecordedHostKey(t *testing.T) {
	ed := testHostSigner(t, false)
	rsaKey := testHostSigner(t, true)
	ca := testHostSigner(t, false)
	rsaCA := testHostSigner(t, true)
	rsaSHA2, err := ssh.NewSignerWithAlgorithms(rsaKey.(ssh.AlgorithmSigner), []string{ssh.KeyAlgoRSASHA256})
	if err != nil {
		t.Fatal(err)
	}
	rsaLegacy, err := ssh.NewSignerWithAlgorithms(rsaKey.(ssh.AlgorithmSigner), []string{ssh.KeyAlgoRSA})
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{HostKeyCheckStrict, HostKeyCheckAcceptNew} {
		for _, tc := range []struct {
			name   string
			keys   []ssh.Signer
			known  ssh.PublicKey
			marker string
			hashed bool
		}{
			{name: "ed25519", keys: []ssh.Signer{rsaKey, ed}, known: ed.PublicKey()},
			{name: "rsa_sha2", keys: []ssh.Signer{rsaSHA2, ed}, known: rsaKey.PublicKey()},
			{name: "rsa_legacy", keys: []ssh.Signer{rsaLegacy, ed}, known: rsaKey.PublicKey()},
			{name: "hashed_nonstandard_port", keys: []ssh.Signer{rsaKey, ed}, known: ed.PublicKey(), hashed: true},
			{name: "ed25519_ca_rsa_host", keys: []ssh.Signer{testHostCertificate(t, rsaKey, ca), ed}, known: ca.PublicKey(), marker: "@cert-authority "},
			{name: "rsa_ca_ed25519_host", keys: []ssh.Signer{testHostCertificate(t, ed, rsaCA), rsaKey}, known: rsaCA.PublicKey(), marker: "@cert-authority "},
		} {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				if tc.name == "rsa_legacy" && fips140.Enabled() {
					t.Skip("SHA-1 host key signatures are disabled in FIPS mode")
				}
				srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true, HostKeys: tc.keys})
				cfg := testConfigForServer(t, srv, policy)
				cfg.SSHHandshakeAttempts = 1
				host := srv.Address
				if tc.hashed {
					host = knownhosts.HashHostname(knownhosts.Normalize(host))
				}
				original := []byte(tc.marker + knownhosts.Line([]string{host}, tc.known) + "\n")
				if err := os.WriteFile(cfg.SSHKnownHostsFile, original, 0600); err != nil {
					t.Fatal(err)
				}
				state := new(multistep.BasicStateBag)
				state.Put("config", cfg)
				state.Put("ui", &testUi{})
				step := new(stepConnect)
				defer step.Cleanup(state)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if action := step.Run(ctx, state); action != multistep.ActionContinue {
					t.Fatalf("Run = %v, error = %v", action, state.Get("error"))
				}
				data, err := os.ReadFile(cfg.SSHKnownHostsFile) // #nosec G304 -- temporary known_hosts fixture
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, original) {
					t.Fatalf("known key was changed or appended: %q", data)
				}
			})
		}
	}
}

func TestSSHConnectRejectsInvalidHostCertificate(t *testing.T) {
	host := testHostSigner(t, true)
	ca := testHostSigner(t, false)
	wrongCA := testHostSigner(t, false)
	expired := *testHostCertificate(t, host, ca).PublicKey().(*ssh.Certificate)
	expired.ValidAfter = 1
	expired.ValidBefore = 2 // Expired immediately after the Unix epoch.
	if err := expired.SignCert(rand.Reader, ca); err != nil {
		t.Fatal(err)
	}
	expiredSigner, err := ssh.NewCertSigner(&expired, host)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range []string{HostKeyCheckStrict, HostKeyCheckAcceptNew} {
		for _, tc := range []struct {
			name string
			key  ssh.Signer
		}{
			{name: "untrusted_ca", key: testHostCertificate(t, host, wrongCA)},
			{name: "expired", key: expiredSigner},
		} {
			t.Run(policy+"/"+tc.name, func(t *testing.T) {
				srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true, HostKeys: []ssh.Signer{tc.key}})
				cfg := testConfigForServer(t, srv, policy)
				cfg.SSHHandshakeAttempts = 1
				original := []byte("@cert-authority " + knownhosts.Line([]string{srv.Address}, ca.PublicKey()) + "\n")
				if err := os.WriteFile(cfg.SSHKnownHostsFile, original, 0600); err != nil {
					t.Fatal(err)
				}
				state := new(multistep.BasicStateBag)
				state.Put("config", cfg)
				state.Put("ui", &testUi{})
				step := new(stepConnect)
				defer step.Cleanup(state)
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if action := step.Run(ctx, state); action != multistep.ActionHalt || state.Get("error") == nil {
					t.Fatalf("invalid certificate accepted: action=%v error=%v", action, state.Get("error"))
				}
				data, err := os.ReadFile(cfg.SSHKnownHostsFile) // #nosec G304 -- temporary known_hosts fixture
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(data, original) {
					t.Fatalf("invalid certificate changed known_hosts: %q", data)
				}
			})
		}
	}
}

func TestSSHConnectHostKeySelectionPreservesPolicy(t *testing.T) {
	ed := testHostSigner(t, false)
	rsaKey := testHostSigner(t, true)
	oldKey := testHostSigner(t, false)
	for _, tc := range []struct {
		name         string
		policy       string
		known        ssh.PublicKey
		marker       string
		wantSuccess  bool
		wantMismatch bool
		wantRevoked  bool
	}{
		{name: "unknown_accept_new", policy: HostKeyCheckAcceptNew, wantSuccess: true},
		{name: "unknown_strict", policy: HostKeyCheckStrict},
		{name: "ip_entry_is_not_dns_entry", policy: HostKeyCheckStrict, known: ed.PublicKey()},
		{name: "off_ignores_file", policy: HostKeyCheckOff, wantSuccess: true},
		{name: "changed_strict", policy: HostKeyCheckStrict, known: oldKey.PublicKey(), wantMismatch: true},
		{name: "changed_accept_new", policy: HostKeyCheckAcceptNew, known: oldKey.PublicKey(), wantMismatch: true},
		{name: "revoked_strict", policy: HostKeyCheckStrict, known: rsaKey.PublicKey(), marker: "@revoked ", wantRevoked: true},
		{name: "revoked_accept_new", policy: HostKeyCheckAcceptNew, known: rsaKey.PublicKey(), marker: "@revoked ", wantRevoked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true, HostKeys: []ssh.Signer{rsaKey, ed}})
			cfg := testConfigForServer(t, srv, tc.policy)
			cfg.SSHHandshakeAttempts = 1
			if tc.name == "ip_entry_is_not_dns_entry" {
				cfg.SSHHost = "localhost"
			}
			var original []byte
			if tc.known != nil {
				original = []byte(tc.marker + knownhosts.Line([]string{srv.Address}, tc.known) + "\n")
			}
			if tc.policy == HostKeyCheckOff {
				original = []byte("not a known_hosts file\n")
			}
			if err := os.WriteFile(cfg.SSHKnownHostsFile, original, 0600); err != nil {
				t.Fatal(err)
			}
			state := new(multistep.BasicStateBag)
			state.Put("config", cfg)
			state.Put("ui", &testUi{})
			step := new(stepConnect)
			defer step.Cleanup(state)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			action := step.Run(ctx, state)
			if tc.wantSuccess {
				if action != multistep.ActionContinue {
					t.Fatalf("Run = %v, error = %v", action, state.Get("error"))
				}
			} else {
				if action != multistep.ActionHalt {
					t.Fatalf("Run = %v, want halt", action)
				}
				gotErr := state.Get("error").(error)
				if tc.name == "unknown_strict" || tc.name == "ip_entry_is_not_dns_entry" {
					var unknown *knownhosts.KeyError
					if !errors.As(gotErr, &unknown) || len(unknown.Want) != 0 {
						t.Fatalf("error = %v, want unknown host", gotErr)
					}
				}
				if tc.wantMismatch {
					var mismatch *knownhosts.KeyError
					if !errors.As(gotErr, &mismatch) || len(mismatch.Want) == 0 {
						t.Fatalf("error = %v, want key mismatch", gotErr)
					}
				}
				if tc.wantRevoked {
					var revoked *knownhosts.RevokedError
					if !errors.As(gotErr, &revoked) {
						t.Fatalf("error = %v, want revoked key", gotErr)
					}
				}
			}
			data, err := os.ReadFile(cfg.SSHKnownHostsFile) // #nosec G304 -- temporary known_hosts fixture
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "unknown_accept_new" {
				if strings.Count(string(data), "\n") != 1 {
					t.Fatalf("recorded unexpected entries: %q", data)
				}
				if err := verifyRecordedKey(cfg.SSHKnownHostsFile, srv.Address, rsaKey.PublicKey()); err != nil {
					t.Fatalf("unknown host did not retain default negotiation: %v", err)
				}
			} else if !bytes.Equal(data, original) {
				t.Fatalf("known_hosts was modified: %q", data)
			}
		})
	}
}
