package lxc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestDialSSHBudgetIncludesDNS(t *testing.T) {
	// A resolver that blocks until its context expires makes a slow dial
	// deterministic without depending on an unreachable external host.
	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	t.Cleanup(func() { net.DefaultResolver = previous })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := dialSSH(ctx, "packer-budget-test.invalid:22", &ssh.ClientConfig{
		User: "root", HostKeyCallback: ssh.InsecureIgnoreHostKey(), // #nosec G106 -- local timeout fixture
	}, time.Now().Add(50*time.Millisecond))
	if err == nil {
		t.Fatal("expected the stalled DNS lookup to fail")
	}
	if ctx.Err() != nil {
		t.Fatalf("dial exhausted the parent context instead of the shorter SSH budget: %v", err)
	}
}

func TestDialSSHStopsDuringHandshake(t *testing.T) {
	for _, name := range []string{"cancelled", "parent deadline"} {
		t.Run(name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = listener.Close() })
			accepted := make(chan net.Conn, 1)
			go func() {
				conn, err := listener.Accept()
				if err == nil {
					accepted <- conn
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() {
				client, err := dialSSH(ctx, listener.Addr().String(), &ssh.ClientConfig{
					User: "root", HostKeyCallback: ssh.InsecureIgnoreHostKey(), // #nosec G106 -- local stalled-handshake fixture
				}, time.Now().Add(5*time.Second))
				if client != nil {
					_ = client.Close()
				}
				result <- err
			}()
			select {
			case conn := <-accepted:
				t.Cleanup(func() { _ = conn.Close() })
			case <-ctx.Done():
				t.Fatal("TCP connection was not established")
			}
			want := context.DeadlineExceeded
			if name == "cancelled" {
				cancel()
				want = context.Canceled
			}
			select {
			case err := <-result:
				if !errors.Is(err, want) {
					t.Fatalf("dialSSH error = %v, want %v", err, want)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("context cancellation did not interrupt the handshake")
			}
		})
	}
}

func TestDialSSHSuccessfulSessionOutlivesConnectContext(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	auth, err := buildSSHClientConfig(context.Background(), &Config{SSHUser: "root", SSHPassword: "secret", SSHHostKeyCheck: HostKeyCheckOff}, &testUi{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deadline := time.Now().Add(time.Second)
	client, err := dialSSH(ctx, srv.Address, auth.Config, deadline)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	cancel()
	// Neither subsequent cancellation nor the old connection deadline may
	// terminate a successfully established session.
	<-time.After(time.Until(deadline) + 10*time.Millisecond)
	ok, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil || !ok {
		t.Fatalf("established session was affected by the connect context: ok=%v err=%v", ok, err)
	}
}
