package lxc

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var errTestAuthRejected = errors.New("test server: authentication rejected")

// testSSHServer is a minimal in-process SSH server. Host key verification and
// authentication ordering cannot be exercised through the mocks in mock_test.go:
// both only take effect during a real handshake.
type testSSHServer struct {
	Address string
	HostKey ssh.PublicKey

	mu       sync.Mutex
	authUsed string
}

func (s *testSSHServer) AuthUsed() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.authUsed
}

func (s *testSSHServer) setAuthUsed(method string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authUsed = method
}

type testSSHServerOptions struct {
	// AcceptAnyAuth accepts every authentication method, which makes it
	// possible to observe which one the client chose.
	AcceptAnyAuth bool
	// AcceptedPublicKey restricts public-key authentication when AcceptAnyAuth is false.
	AcceptedPublicKey ssh.PublicKey
	// HostKeys overrides the generated host key when testing negotiation.
	HostKeys []ssh.Signer
}

func startTestSSHServer(t *testing.T, opts testSSHServerOptions) *testSSHServer {
	t.Helper()

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("building host signer: %v", err)
	}
	if len(opts.HostKeys) > 0 {
		hostSigner = opts.HostKeys[0]
	}

	srv := &testSSHServer{HostKey: hostSigner.PublicKey()}

	config := &ssh.ServerConfig{}
	if opts.AcceptAnyAuth {
		config.PublicKeyCallback = func(ssh.ConnMetadata, ssh.PublicKey) (*ssh.Permissions, error) {
			srv.setAuthUsed("publickey")
			return nil, nil
		}
		config.PasswordCallback = func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) {
			srv.setAuthUsed("password")
			return nil, nil
		}
	} else {
		config.PublicKeyCallback = func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if opts.AcceptedPublicKey != nil && bytes.Equal(key.Marshal(), opts.AcceptedPublicKey.Marshal()) {
				srv.setAuthUsed("publickey")
				return nil, nil
			}
			return nil, errTestAuthRejected
		}
	}
	if len(opts.HostKeys) == 0 {
		config.AddHostKey(hostSigner)
	} else {
		for _, signer := range opts.HostKeys {
			config.AddHostKey(signer)
		}
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	srv.Address = listener.Addr().String()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go srv.serveConn(conn, config)
		}
	}()

	return srv
}

// fullAuthAgainstServer dials the server using auth and returns the handshake
// error, if any.
func fullAuthAgainstServer(t *testing.T, srv *testSSHServer, auth *sshAuth) error {
	t.Helper()
	client, err := dialSSH(context.Background(), srv.Address, auth.Config, time.Now().Add(sshDialTimeout))
	if err != nil {
		return err
	}
	return client.Close()
}

// startTestAgent serves an in-process SSH agent on a temporary unix socket and
// points SSH_AUTH_SOCK at it, so agent authentication can be exercised without
// a real ssh-agent.
func startTestAgent(t *testing.T, keyPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("test agent uses a unix socket")
	}

	keyBytes, err := os.ReadFile(keyPath) // #nosec G304 -- path is the test fixture key path, not untrusted input
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ssh.ParseRawPrivateKey(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: parsed}); err != nil {
		t.Fatal(err)
	}

	sock := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	t.Setenv("SSH_AUTH_SOCK", sock)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_ = agent.ServeAgent(keyring, c)
			}(conn)
		}
	}()
}

func (s *testSSHServer) serveConn(raw net.Conn, config *ssh.ServerConfig) {
	conn, chans, reqs, err := ssh.NewServerConn(raw, config)
	if err != nil {
		_ = raw.Close()
		return
	}
	defer func() { _ = conn.Close() }()

	go func() {
		for req := range reqs {
			if req.WantReply {
				_ = req.Reply(req.Type == "keepalive@openssh.com", nil)
			}
		}
	}()
	for ch := range chans {
		_ = ch.Reject(ssh.Prohibited, "no channels in tests")
	}
}
