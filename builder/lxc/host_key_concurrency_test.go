package lxc

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func hostKeyForConcurrencyTest(t *testing.T) ssh.PublicKey {
	t.Helper()
	signer, err := parsePrivateKeyFile(writeKey(t, ""), "")
	if err != nil {
		t.Fatal(err)
	}
	return signer.PublicKey()
}

func TestHostKeyAcceptNewStaleCallbacks(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("changed=%t", changed), func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "known_hosts")
			address := "127.0.0.1:22"
			first, err := acceptNewHostKeyCallback(context.Background(), file, address, &testUi{})
			if err != nil {
				t.Fatal(err)
			}
			second, err := acceptNewHostKeyCallback(context.Background(), file, address, &testUi{})
			if err != nil {
				t.Fatal(err)
			}
			key := hostKeyForConcurrencyTest(t)
			remote := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}
			if err := first(address, remote, key); err != nil {
				t.Fatal(err)
			}
			other := key
			if changed {
				other = hostKeyForConcurrencyTest(t)
			}
			err = second(address, remote, other)
			if changed {
				var keyErr *knownhosts.KeyError
				if !errors.As(err, &keyErr) || len(keyErr.Want) == 0 {
					t.Fatalf("changed key error = %v, want a key mismatch", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(file) // #nosec G304 -- file is this test's temporary known_hosts fixture
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(string(content), "\n"); got != 1 {
				t.Fatalf("recorded %d entries, want exactly one", got)
			}
		})
	}
}

func TestHostKeyAcceptNewConcurrentCallbacks(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprintf("different_keys=%t", changed), func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "known_hosts")
			address := "127.0.0.1:22"
			callback, err := acceptNewHostKeyCallback(context.Background(), file, address, &testUi{})
			if err != nil {
				t.Fatal(err)
			}
			const builds = 12
			keys := make([]ssh.PublicKey, builds)
			key := hostKeyForConcurrencyTest(t)
			for i := range keys {
				keys[i] = key
				if changed {
					keys[i] = hostKeyForConcurrencyTest(t)
				}
			}
			start := make(chan struct{})
			results := make(chan error, builds)
			for _, key := range keys {
				go func() {
					<-start
					results <- callback(address, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key)
				}()
			}
			close(start)
			accepted := 0
			for range builds {
				err := <-results
				if err == nil {
					accepted++
					continue
				}
				var keyErr *knownhosts.KeyError
				if !changed || !errors.As(err, &keyErr) || len(keyErr.Want) == 0 {
					t.Errorf("callback error = %v, want a key mismatch only for competing keys", err)
				}
			}
			want := builds
			if changed {
				want = 1
			}
			if accepted != want {
				t.Errorf("accepted %d builds, want %d", accepted, want)
			}
			content, err := os.ReadFile(file) // #nosec G304 -- file is this test's temporary known_hosts fixture
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(string(content), "\n"); got != 1 {
				t.Errorf("recorded %d entries, want exactly one", got)
			}
		})
	}
}

func TestHostKeyAcceptNewLockHonoursConnectContext(t *testing.T) {
	for _, name := range []string{"cancelled", "budget exhausted"} {
		t.Run(name, func(t *testing.T) {
			srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
			cfg := testConfigForServer(t, srv, HostKeyCheckAcceptNew)
			cfg.SSHHandshakeAttempts = 1
			cfg.SSHTimeout = "5s"
			if name == "budget exhausted" {
				cfg.SSHTimeout = "50ms"
			}
			if err := ensureKnownHostsFile(cfg.SSHKnownHostsFile); err != nil {
				t.Fatal(err)
			}
			lock := flock.New(cfg.SSHKnownHostsFile + ".lock")
			if err := lock.Lock(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = lock.Close() })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			state := new(multistep.BasicStateBag)
			state.Put("config", cfg)
			state.Put("ui", &testUi{})
			step := &stepConnect{}
			t.Cleanup(func() { step.Cleanup(state) })
			result := make(chan multistep.StepAction, 1)
			go func() { result <- step.Run(ctx, state) }()
			want := context.DeadlineExceeded
			if name == "cancelled" {
				select {
				case action := <-result:
					t.Fatalf("Run returned %v while known_hosts was locked", action)
				case <-time.After(100 * time.Millisecond):
				}
				cancel()
				want = context.Canceled
			}
			select {
			case action := <-result:
				if action != multistep.ActionHalt {
					t.Fatalf("Run = %v, want ActionHalt", action)
				}
				if err, ok := state.Get("error").(error); !ok || !errors.Is(err, want) {
					t.Fatalf("error = %v, want %v", state.Get("error"), want)
				}
			case <-time.After(time.Second):
				t.Fatal("lock wait ignored the connect context")
			}
		})
	}
}

func TestHostKeyAcceptNewSymlinkUsesSharedLock(t *testing.T) {
	file := filepath.Join(t.TempDir(), "known_hosts")
	if err := ensureKnownHostsFile(file); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(file), "alias")
	if err := os.Symlink(file, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	lock := flock.New(file + ".lock")
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	address := "127.0.0.1:22"
	callback, err := acceptNewHostKeyCallback(ctx, alias, address, &testUi{})
	if err != nil {
		t.Fatal(err)
	}
	err = callback(address, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, hostKeyForConcurrencyTest(t))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("symlink bypassed the shared lock: %v", err)
	}
}

func TestHostKeyAcceptNewKnownKeyNeedsNoWriteLock(t *testing.T) {
	file := filepath.Join(t.TempDir(), "known_hosts")
	address := "127.0.0.1:22"
	key := hostKeyForConcurrencyTest(t)
	if err := appendKnownHostEntry(file, address, key); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	callback, err := acceptNewHostKeyCallback(ctx, file, address, &testUi{})
	if err != nil {
		t.Fatal(err)
	}
	if err := callback(address, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("verifying a recorded key created a lock file: %v", err)
	}
}

func TestHostKeyAcceptNewRekeyAfterConnect(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	cfg := testConfigForServer(t, srv, HostKeyCheckAcceptNew)
	ctx, cancel := sshConnectContext(context.Background(), time.Now().Add(2*time.Second))
	defer cancel()
	auth, err := buildSSHClientConfig(ctx, cfg, &testUi{})
	if err != nil {
		t.Fatal(err)
	}
	// Force rekeying after a small amount of traffic. The host-key callback
	// is also used for rekeys, after the connection setup context is done.
	auth.Config.RekeyThreshold = 256
	client, err := dialSSH(ctx, cfg.sshAddress(), auth.Config, time.Now().Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	cancel()
	requestCtx, stop := context.WithTimeout(context.Background(), 4*time.Second)
	defer stop()
	stopClosing := context.AfterFunc(requestCtx, func() { _ = client.Close() })
	defer stopClosing()
	for range 5 {
		ok, _, err := client.SendRequest("keepalive@openssh.com", true, make([]byte, 4096))
		if err != nil || !ok {
			t.Fatalf("rekey after connection setup failed: ok=%v error=%v", ok, err)
		}
	}
}

func TestHostKeyAcceptNewCrossProcessLock(t *testing.T) {
	const marker = "PACKER_LXC_TEST_LOCK_PATH"
	if file := os.Getenv(marker); file != "" {
		lock := flock.New(file)
		if err := lock.Lock(); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = lock.Close() }()
		fmt.Println("locked")
		if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
			t.Fatal(err)
		}
		return
	}
	file := filepath.Join(t.TempDir(), "known_hosts")
	address := "127.0.0.1:22"
	callback, err := acceptNewHostKeyCallback(context.Background(), file, address, &testUi{})
	if err != nil {
		t.Fatal(err)
	}
	key := hostKeyForConcurrencyTest(t)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestHostKeyAcceptNewCrossProcessLock$") // #nosec G204 -- executable is this test binary, arguments are fixed
	cmd.Env = append(os.Environ(), marker+"="+file+".lock")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = fmt.Fprintln(stdin, "release")
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("lock helper: %v: %s", err, stderr.String())
		}
	}()
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "locked\n" {
		t.Fatalf("lock helper readiness = %q, error = %v", line, err)
	}
	result := make(chan error, 1)
	go func() { result <- callback(address, &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 22}, key) }()
	select {
	case err := <-result:
		t.Fatalf("callback returned before the other process unlocked: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := fmt.Fprintln(stdin, "release"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("callback did not continue after the other process unlocked")
	}
}
