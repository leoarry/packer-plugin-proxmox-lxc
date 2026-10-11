package lxc

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	// sshDialTimeout bounds a single connection attempt, including the
	// handshake. ssh_timeout bounds the whole retry loop in stepConnect.
	sshDialTimeout = 30 * time.Second
	// sshRetryInterval matches the interval the packer-plugin-sdk uses between
	// connection attempts.
	sshRetryInterval = 5 * time.Second
)

// sshConnectContext gives connection setup and host-key verification the same
// attempt budget, including lock waits before the handshake completes.
func sshConnectContext(ctx context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	if attemptDeadline := time.Now().Add(sshDialTimeout); attemptDeadline.Before(deadline) {
		deadline = attemptDeadline
	}
	return context.WithDeadline(ctx, deadline)
}

// dialSSH bounds DNS, TCP dialing and the SSH handshake by the earliest of
// sshDialTimeout, deadline and the caller's context deadline. Cancellation
// closes an incomplete connection but does not affect an established session.
func dialSSH(ctx context.Context, address string, config *ssh.ClientConfig, deadline time.Time) (*ssh.Client, error) {
	if !time.Now().Before(deadline) {
		return nil, fmt.Errorf("ssh connect budget exhausted before dialing %s", address)
	}
	connectCtx, cancel := sshConnectContext(ctx, deadline)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(connectCtx, "tcp", address)
	if err != nil {
		return nil, err
	}
	// NewClientConn has no context parameter. Closing its underlying socket
	// interrupts a blocked handshake when the shared connect context ends.
	stopCancellation := context.AfterFunc(connectCtx, func() { _ = conn.Close() })
	defer stopCancellation()

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		_ = conn.Close()
		if ctxErr := connectCtx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	// Detach cancellation before returning ownership of the live connection.
	// If the callback already started, the connection cannot be used safely.
	if !stopCancellation() || connectCtx.Err() != nil {
		_ = sshConn.Close()
		return nil, connectCtx.Err()
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}
