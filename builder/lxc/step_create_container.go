package lxc

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// maxCTIDConflictRetries bounds how many times an auto-assigned CTID is
// re-rolled after losing a race with a concurrent build targeting the
// same Proxmox host (see fetchNextCTID for why this race is possible).
const maxCTIDConflictRetries = 5

type stepCreateContainer struct{}

func (s *stepCreateContainer) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	comm := state.Get("communicator").(CommandRunner)
	ctid := state.Get("ctid").(string)
	autoAssigned := config.CTID == ""

	for attempt := 0; ; attempt++ {
		err := comm.RunCommand(ctx, fmt.Sprintf("pct status %s", ctid), nil, nil)
		if err == nil {
			if !autoAssigned {
				ui.Say(fmt.Sprintf("Container %s already exists, reusing", ctid))
				state.Put("container_reused", true)
				return multistep.ActionContinue
			}

			// An auto-assigned CTID should never already have a
			// container — pvesh's "next free id" is only ever supposed
			// to name an unused one. Seeing one here almost certainly
			// means a concurrent build just won the race for this same
			// ID. Do NOT treat this as "reuse": that would silently
			// point this build at another build's in-progress container.
			// Re-roll instead, exactly like a create conflict below.
			if attempt < maxCTIDConflictRetries {
				ui.Say(fmt.Sprintf("CTID %s was claimed by a concurrent build, picking a new CTID and retrying...", ctid))
				time.Sleep(ctidRetryBackoff(attempt))

				newCTID, ferr := fetchNextCTID(ctx, comm)
				if ferr != nil {
					state.Put("error", fmt.Errorf("CTID %s is already in use and failed to get a new CTID: %w", ctid, ferr))
					return multistep.ActionHalt
				}
				ctid = newCTID
				state.Put("ctid", ctid)
				continue
			}

			state.Put("error", fmt.Errorf("CTID %s is already in use after %d retries", ctid, maxCTIDConflictRetries))
			return multistep.ActionHalt
		}

		ui.Say(fmt.Sprintf("Creating container %s...", ctid))

		unprivileged := "1"
		if !config.IsUnprivileged() {
			unprivileged = "0"
		}

		// Fixed: --rootfs uses config.Storage for pool and config.RootfsSize for size
		cmd := fmt.Sprintf(
			"pct create %s %s --unprivileged %s --features %s --hostname builder-%s --storage %s --rootfs %s:%s --memory %d --cores %d --net0 %s --password '%s'",
			ctid,
			config.Template,
			unprivileged,
			config.Features,
			ctid,
			config.Storage,
			config.Storage,
			config.RootfsSize,
			config.Memory,
			config.Cores,
			config.NetworkConfig(),
			config.RootPassword,
		)

		var stderr bytes.Buffer
		err = comm.RunCommand(ctx, cmd, nil, &stderr)
		if err == nil {
			state.Put("container_reused", false)
			ui.Say(fmt.Sprintf("Container %s created", ctid))
			return multistep.ActionContinue
		}

		// A concurrent build may have claimed this auto-assigned CTID
		// between fetchNextCTID and this create call (pvesh's "next free
		// id" is a point-in-time query, not a reservation). Re-roll and
		// retry rather than failing the whole build outright.
		if autoAssigned && attempt < maxCTIDConflictRetries && isCTIDConflict(stderr.String()) {
			ui.Say(fmt.Sprintf("CTID %s was claimed by a concurrent build, picking a new CTID and retrying...", ctid))
			time.Sleep(ctidRetryBackoff(attempt))

			newCTID, ferr := fetchNextCTID(ctx, comm)
			if ferr != nil {
				state.Put("error", fmt.Errorf("pct create failed: %w (and failed to get a new CTID: %v)", err, ferr))
				return multistep.ActionHalt
			}
			ctid = newCTID
			state.Put("ctid", ctid)
			continue
		}

		state.Put("error", fmt.Errorf("pct create failed: %w", err))
		return multistep.ActionHalt
	}
}

// isCTIDConflict reports whether a `pct create` failure looks like a CTID
// collision (e.g. "CT 101 already exists" / "configuration file ... already
// exists") rather than some other, non-retryable failure.
func isCTIDConflict(stderr string) bool {
	return strings.Contains(strings.ToLower(stderr), "already exists")
}

// ctidRetryBackoff returns a short, jittered delay before retrying with a
// new CTID, to reduce the odds of repeatedly colliding with the same
// concurrent builds.
func ctidRetryBackoff(attempt int) time.Duration {
	base := 100 * time.Millisecond * time.Duration(attempt+1)
	jitter := time.Duration(rand.Intn(200)) * time.Millisecond
	return base + jitter
}

func (s *stepCreateContainer) Cleanup(state multistep.StateBag) {
	// If container was already destroyed by stepDestroyContainer, skip cleanup.
	if _, ok := state.GetOk("container_destroyed"); ok {
		return
	}

	// If container was reused (existed before our build), don't destroy it.
	if reused, ok := state.GetOk("container_reused"); ok && reused.(bool) {
		return
	}

	// If container was successfully converted to a Proxmox CT template,
	// it *is* the artifact — don't destroy it.
	if templated, ok := state.GetOk("container_templated"); ok && templated.(bool) {
		return
	}

	ctid, ok := state.GetOk("ctid")
	if !ok {
		return
	}
	ctidStr := ctid.(string)

	// Get the command runner for the Proxmox host.
	// After stepSetupContainerComm, the host communicator is saved as "proxmox_comm".
	// Before that, the communicator in "communicator" is the sshCommunicator.
	var cmdRunner CommandRunner
	if comm, ok := state.GetOk("proxmox_comm"); ok {
		cmdRunner = comm.(CommandRunner)
	} else if comm, ok := state.GetOk("communicator"); ok {
		if cr, ok := comm.(CommandRunner); ok {
			cmdRunner = cr
		}
	}
	if cmdRunner == nil {
		return
	}

	if ui, ok := state.GetOk("ui"); ok {
		ui.(packersdk.Ui).Say(fmt.Sprintf("Cleaning up container %s due to failure...", ctidStr))
	}

	// Best-effort cleanup: stop and destroy the container on the Proxmox host.
	_ = cmdRunner.RunCommand(context.Background(), fmt.Sprintf("pct stop %s || true", ctidStr), nil, nil)
	_ = cmdRunner.RunCommand(context.Background(), fmt.Sprintf("pct destroy %s --purge", ctidStr), nil, nil)
	_ = cmdRunner.RunCommand(context.Background(), fmt.Sprintf("rm -f /tmp/vzdump-lxc-%s-*.log 2>/dev/null || true", ctidStr), nil, nil)
}
