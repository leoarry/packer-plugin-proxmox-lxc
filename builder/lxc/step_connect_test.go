package lxc

import (
	"context"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

// unusedPort returns a host and port that refuse connections immediately.
func unusedPort(t *testing.T) (string, int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, portStr, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return host, port
}

func TestStepConnectSucceedsAgainstTestServer(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	host, portStr, _ := net.SplitHostPort(srv.Address)
	port, _ := strconv.Atoi(portStr)

	config := &Config{
		SSHHost:              host,
		SSHPort:              port,
		SSHUser:              "root",
		SSHPassword:          "secret",
		SSHHostKeyCheck:      HostKeyCheckAcceptNew,
		SSHKnownHostsFile:    filepath.Join(t.TempDir(), "known_hosts"),
		SSHTimeout:           "5s",
		SSHHandshakeAttempts: 3,
	}
	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", &testUi{})

	step := &stepConnect{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("Run() = %v, want ActionContinue (error: %v)", action, state.Get("error"))
	}
	if _, ok := state.GetOk("communicator"); !ok {
		t.Error("communicator was not put in the state")
	}
	if _, ok := state.GetOk("ssh_client"); !ok {
		t.Error("ssh_client was not put in the state")
	}
	step.Cleanup(state)
}

func TestStepConnectRetriesThenHonoursTimeout(t *testing.T) {
	host, port := unusedPort(t)

	config := &Config{
		SSHHost:              host,
		SSHPort:              port,
		SSHUser:              "root",
		SSHPassword:          "secret",
		SSHHostKeyCheck:      HostKeyCheckOff,
		SSHTimeout:           "200ms",
		SSHHandshakeAttempts: 5,
	}
	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", &testUi{})

	start := time.Now()
	step := &stepConnect{}
	if action := step.Run(context.Background(), state); action != multistep.ActionHalt {
		t.Fatalf("Run() = %v, want ActionHalt", action)
	}
	elapsed := time.Since(start)

	err, ok := state.Get("error").(error)
	if !ok || !strings.Contains(err.Error(), "failed after") {
		t.Fatalf("error = %v, want it to report the failed attempts", state.Get("error"))
	}
	// The retry interval (5s) is longer than the timeout (200ms): the step must
	// give up instead of sleeping past its deadline.
	if elapsed > 3*time.Second {
		t.Errorf("Run() took %s, want it to give up promptly at the ssh_timeout deadline", elapsed)
	}
}

func TestStepConnectStopsWhenContextIsCancelled(t *testing.T) {
	host, port := unusedPort(t)

	config := &Config{
		SSHHost:              host,
		SSHPort:              port,
		SSHUser:              "root",
		SSHPassword:          "secret",
		SSHHostKeyCheck:      HostKeyCheckOff,
		SSHTimeout:           "30s",
		SSHHandshakeAttempts: 10,
	}
	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", &testUi{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	step := &stepConnect{}
	if action := step.Run(ctx, state); action != multistep.ActionHalt {
		t.Fatalf("Run() = %v, want ActionHalt on a cancelled context", action)
	}
	if err, ok := state.Get("error").(error); !ok || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error = %v, want a cancellation error", state.Get("error"))
	}
}

func TestStepConnectBudgetCappedOnBlackholedHost(t *testing.T) {
	// A listener that accepts TCP but never completes the handshake: the
	// flat 30 s deadline would hang here; the ssh_timeout budget must cap it.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn // never speak, never close
		}
	}()
	host, portStr, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	config := &Config{
		SSHHost:              host,
		SSHPort:              port,
		SSHUser:              "root",
		SSHPassword:          "secret",
		SSHHostKeyCheck:      HostKeyCheckOff,
		SSHTimeout:           "500ms",
		SSHHandshakeAttempts: 3,
	}
	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", &testUi{})

	start := time.Now()
	step := &stepConnect{}
	if action := step.Run(context.Background(), state); action != multistep.ActionHalt {
		t.Fatalf("Run() = %v, want ActionHalt", action)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Run() took %s with a 500ms budget: the per-attempt deadline is not capped by ssh_timeout", elapsed)
	}
}

func TestStepConnectReleasesTheAgentConnection(t *testing.T) {
	srv := startTestSSHServer(t, testSSHServerOptions{AcceptAnyAuth: true})
	keyPath := writeKey(t, "")
	startTestAgent(t, keyPath)

	host, portStr, _ := net.SplitHostPort(srv.Address)
	port, _ := strconv.Atoi(portStr)

	config := &Config{
		SSHHost:              host,
		SSHPort:              port,
		SSHUser:              "root",
		SSHKeyPath:           keyPath,
		SSHAgentAuth:         true,
		SSHHostKeyCheck:      HostKeyCheckOff,
		SSHTimeout:           "5s",
		SSHHandshakeAttempts: 3,
	}
	state := new(multistep.BasicStateBag)
	state.Put("config", config)
	state.Put("ui", &testUi{})

	step := &stepConnect{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("Run() = %v, want ActionContinue (error: %v)", action, state.Get("error"))
	}
	if _, ok := state.GetOk("ssh_agent_conn"); !ok {
		t.Error("ssh_agent_conn was not put in the state")
	}
	step.Cleanup(state)
}
