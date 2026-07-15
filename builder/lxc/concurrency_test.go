package lxc

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

// fakeProxmoxHost is a minimal, concurrency-safe stand-in for a real
// Proxmox host, used to reproduce GitHub issue #5: parallel builds racing
// on `pvesh get /cluster/nextid` and landing on the same CTID.
type fakeProxmoxHost struct {
	mu         sync.Mutex
	nextID     int
	containers map[string]bool
}

func newFakeProxmoxHost(startID int) *fakeProxmoxHost {
	return &fakeProxmoxHost{nextID: startID, containers: map[string]bool{}}
}

func (h *fakeProxmoxHost) RunCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	switch {
	case command == "pvesh get /cluster/nextid":
		// Deliberately NOT reserving anything here, mirroring real Proxmox:
		// this is a point-in-time read, so concurrent callers can (and, in
		// the bug report, did) observe the same "next free" id.
		h.mu.Lock()
		id := h.nextID
		for h.containers[strconv.Itoa(id)] {
			id++
		}
		h.mu.Unlock()
		if stdout != nil {
			_, _ = fmt.Fprintf(stdout, "%d\n", id)
		}
		return nil

	case strings.HasPrefix(command, "pct status "):
		id := strings.TrimPrefix(command, "pct status ")
		h.mu.Lock()
		exists := h.containers[id]
		h.mu.Unlock()
		if exists {
			return nil
		}
		return fmt.Errorf("container %s does not exist", id)

	case strings.HasPrefix(command, "pct create "):
		fields := strings.Fields(command)
		id := fields[2]
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.containers[id] {
			if stderr != nil {
				_, _ = fmt.Fprintf(stderr, "unable to create CT %s - already exists\n", id)
			}
			return fmt.Errorf("pct create failed: CT %s already exists", id)
		}
		h.containers[id] = true
		return nil
	}
	return nil
}

// TestConcurrentBuilds_CTIDConflict_ResolvedByRetry reproduces the exact
// scenario from GitHub issue #5: two "packer build" processes targeting
// the same Proxmox host, both auto-assigning a CTID at roughly the same
// time. Before the fix, both would get the same CTID from `pvesh get
// /cluster/nextid` and one would fail outright in stepCreateContainer.
// With the fix, the loser of the race retries with a freshly fetched CTID
// instead of failing the build.
func TestConcurrentBuilds_CTIDConflict_ResolvedByRetry(t *testing.T) {
	host := newFakeProxmoxHost(300)

	const numBuilds = 2
	var wgFetched sync.WaitGroup
	wgFetched.Add(numBuilds)
	release := make(chan struct{})

	results := make([]string, numBuilds)
	errs := make([]error, numBuilds)
	var wgDone sync.WaitGroup
	wgDone.Add(numBuilds)

	for i := 0; i < numBuilds; i++ {
		i := i
		go func() {
			defer wgDone.Done()

			config := &Config{
				Template:     "local:vztmpl/ubuntu-22.04.tar.gz",
				Unprivileged: true,
				Storage:      "local-lvm",
				Memory:       512,
				Cores:        1,
				RootfsSize:   "2",
				RootPassword: "x",
				Bridge:       "vmbr0",
				Features:     "nesting=1",
				NetworkIP:    "dhcp",
			}

			state := new(multistep.BasicStateBag)
			state.Put("ui", &testUi{})
			state.Put("communicator", host)
			state.Put("config", config)

			getStep := &stepGetCTID{}
			if action := getStep.Run(context.Background(), state); action != multistep.ActionContinue {
				errs[i] = fmt.Errorf("stepGetCTID did not continue")
				wgFetched.Done()
				return
			}
			wgFetched.Done()

			// Hold until every build has fetched a CTID, maximizing the
			// chance they collided on the same "next free" id, exactly
			// like the two parallel builds in the bug report.
			<-release

			createStep := &stepCreateContainer{}
			action := createStep.Run(context.Background(), state)
			if action != multistep.ActionContinue {
				if errVal, ok := state.GetOk("error"); ok {
					errs[i] = errVal.(error)
				} else {
					errs[i] = fmt.Errorf("stepCreateContainer halted with no error in state")
				}
				return
			}
			results[i] = state.Get("ctid").(string)
		}()
	}

	wgFetched.Wait()
	close(release)
	wgDone.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("build %d failed: %v", i, err)
		}
	}
	if results[0] == "" || results[1] == "" {
		t.Fatalf("expected both builds to succeed with a CTID, got %v", results)
	}
	if results[0] == results[1] {
		t.Errorf("expected distinct CTIDs for concurrent builds, both got %q", results[0])
	}
}
