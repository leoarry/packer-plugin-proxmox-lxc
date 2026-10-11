package lxc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/flock"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostsPath returns the expanded ssh_known_hosts_file value.
func knownHostsPath(cfg *Config) (string, error) {
	return expandUserPath(cfg.SSHKnownHostsFile)
}

// sshAddress returns the host:port form used both for dialing and for the
// known_hosts entry.
func (cfg *Config) sshAddress() string {
	return net.JoinHostPort(cfg.SSHHost, strconv.Itoa(cfg.SSHPort))
}

// hostKeyRemediation returns the message shown when a host is unknown. It
// contains the exact command an operator can copy to record the key.
func hostKeyRemediation(address, file string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		host, port = address, "22"
	}
	scan := fmt.Sprintf("ssh-keyscan -H %s >> %s", host, file)
	if port != "22" {
		scan = fmt.Sprintf("ssh-keyscan -H -p %s %s >> %s", port, host, file)
	}
	return fmt.Sprintf("host key is unknown for %s; verify it out of band then run %q, "+
		"or set ssh_host_key_check = %q", address, scan, HostKeyCheckAcceptNew)
}

// hostKeyCallback builds the ssh.HostKeyCallback implementing
// ssh_host_key_check.
func hostKeyCallback(ctx context.Context, cfg *Config, ui packersdk.Ui) (ssh.HostKeyCallback, error) {
	address := cfg.sshAddress()

	switch cfg.SSHHostKeyCheck {
	case HostKeyCheckOff:
		ui.Say(fmt.Sprintf("Warning: ssh_host_key_check is %q, the connection to %s is not "+
			"verified and can be intercepted (man-in-the-middle).", HostKeyCheckOff, address))
		return ssh.InsecureIgnoreHostKey(), nil // #nosec G106 -- explicit ssh_host_key_check="off" opt-out, warned in Prepare and at connect

	case HostKeyCheckStrict:
		file, err := knownHostsPath(cfg)
		if err != nil {
			return nil, err
		}
		callback, err := knownhosts.New(file)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("%s does not exist: %s", file, hostKeyRemediation(address, file))
			}
			return nil, fmt.Errorf("reading %s: %w", file, err)
		}
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			err := callback(hostname, remote, key)
			if err == nil {
				return nil
			}
			var keyErr *knownhosts.KeyError
			if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
				return fmt.Errorf("%w — %s", err, hostKeyRemediation(address, file))
			}
			// A key mismatch passes through raw: suggesting ssh-keyscan there
			// would advise re-recording the new key, defeating the check.
			return err
		}, nil

	case HostKeyCheckAcceptNew:
		file, err := knownHostsPath(cfg)
		if err != nil {
			return nil, err
		}
		return acceptNewHostKeyCallback(ctx, file, address, ui)

	default:
		// Config.Prepare rejects this, but Config values built directly (in
		// tests or by other callers) must not silently fall back to a policy.
		return nil, fmt.Errorf("ssh_host_key_check must be one of %s, got %q",
			strings.Join([]string{HostKeyCheckAcceptNew, HostKeyCheckOff, HostKeyCheckStrict}, ", "),
			cfg.SSHHostKeyCheck)
	}
}

// acceptNewHostKeyCallback accepts a host key that is not recorded yet, records
// it, and rejects everything else. A key that changed is always fatal: that is
// the case this option exists to catch. Unknown keys are rechecked under a
// sibling file lock before appending and verifying, so concurrent builds
// cannot record competing keys.
func acceptNewHostKeyCallback(ctx context.Context, file, address string, ui packersdk.Ui) (ssh.HostKeyCallback, error) {
	if err := ensureKnownHostsFile(file); err != nil {
		return nil, err
	}
	// Symlink aliases must select the same lock and stay bound to that file
	// while the callback is waiting for another build to finish.
	resolvedFile, err := filepath.EvalSymlinks(file)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", file, err)
	}
	file = resolvedFile
	if _, err := knownhosts.New(file); err != nil {
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}
	cannotRecord := func(err error) error {
		return fmt.Errorf("host key for %s is unknown and cannot be recorded in %s: %w "+
			"(point ssh_known_hosts_file at a writable file in a writable directory, use ssh_host_key_check = %q, "+
			"or explicitly accept the risk with %q)",
			address, file, err, HostKeyCheckStrict, HostKeyCheckOff)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		lookup := func() error {
			callback, err := knownhosts.New(file)
			if err != nil {
				return fmt.Errorf("reading %s: %w", file, err)
			}
			return callback(hostname, remote, key)
		}
		// Recorded keys need no write lock. This also keeps rekey verification
		// working after the initial connection context has been cancelled.
		err := lookup()
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) || len(keyErr.Want) > 0 {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// Each invocation needs its own lock handle: sharing a Flock object
		// would treat another goroutine's held lock as already acquired.
		// A separate lock file avoids mandatory data-file locks on Windows.
		// Keep it on disk: deleting it could give concurrent writers different
		// lock handles for the same known-hosts file.
		lock := flock.New(file + ".lock")
		defer func() { _ = lock.Close() }()
		locked, err := lock.TryLockContext(ctx, 10*time.Millisecond)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return cannotRecord(fmt.Errorf("locking %s.lock: %w", file, err))
		}
		if !locked {
			return cannotRecord(fmt.Errorf("could not lock %s.lock", file))
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		// Another build may have recorded a key while this one waited.
		err = lookup()
		if err == nil {
			return nil
		}
		if !errors.As(err, &keyErr) || len(keyErr.Want) > 0 {
			// A recorded key that does not match, or an unreadable file.
			return err
		}

		if appendErr := appendKnownHostEntry(file, address, key); appendErr != nil {
			return cannotRecord(appendErr)
		}
		if verifyErr := verifyRecordedKey(file, address, key); verifyErr != nil {
			return verifyErr
		}

		ui.Say(fmt.Sprintf("Warning: host key for %s was unknown and has been recorded in %s "+
			"(ssh_host_key_check = %q); it will be verified from now on.",
			address, file, HostKeyCheckAcceptNew))
		return nil
	}, nil
}

// ensureKnownHostsFile creates an empty known_hosts file if it does not exist,
// so the first connection can record a key.
func ensureKnownHostsFile(file string) error {
	if _, err := os.Stat(file); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", file, err)
	}
	if dir := filepath.Dir(file); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644) // #nosec G304,G302 -- path is the operator-configured ssh_known_hosts_file; 0644 is the standard known_hosts mode (public keys)
	if err != nil {
		return fmt.Errorf("creating %s: %w", file, err)
	}
	return f.Close()
}

// appendKnownHostEntry records one host key, in the format OpenSSH uses.
func appendKnownHostEntry(file, address string, key ssh.PublicKey) error {
	if err := ensureKnownHostsFile(file); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o644) // #nosec G304,G302 -- path is the operator-configured ssh_known_hosts_file; 0644 is the standard known_hosts mode (public keys)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(knownhosts.Line([]string{knownhosts.Normalize(address)}, key) + "\n"); err != nil {
		return err
	}
	return f.Sync()
}

// verifyRecordedKey re-reads the file and replays the lookup, so a bad entry is
// reported instead of being trusted.
func verifyRecordedKey(file, address string, key ssh.PublicKey) error {
	callback, err := knownhosts.New(file)
	if err != nil {
		return fmt.Errorf("rereading %s: %w", file, err)
	}
	if err := callback(address, &net.TCPAddr{}, key); err != nil {
		return fmt.Errorf("recorded host key in %s does not verify: %w", file, err)
	}
	return nil
}
