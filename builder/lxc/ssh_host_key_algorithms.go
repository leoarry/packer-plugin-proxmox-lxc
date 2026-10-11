package lxc

import (
	"bytes"
	"crypto/fips140"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// knownHostKeyAlgorithms restricts negotiation to key types already trusted
// for this address. Unknown hosts keep SSH's default algorithms.
func knownHostKeyAlgorithms(file, address string) ([]string, error) {
	callback, err := knownhosts.New(file)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}
	// Probe only the read-only callback. An accept-new callback could record
	// this placeholder as a new host key.
	var keyErr *knownhosts.KeyError
	err = callback(address, &net.TCPAddr{IP: net.IPv4zero}, hostKeyLookup{})
	if !errors.As(err, &keyErr) {
		return nil, fmt.Errorf("looking up host keys for %s: %w", address, err)
	}
	if len(keyErr.Want) == 0 {
		return nil, nil
	}

	data, err := os.ReadFile(file) // #nosec G304 -- file is the configured ssh_known_hosts_file
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", file, err)
	}
	lines := bytes.Split(data, []byte("\n"))
	allowed := make(map[string]bool)
	certAuthority := false
	for _, known := range keyErr.Want {
		if known.Line < 1 || known.Line > len(lines) {
			return nil, fmt.Errorf("%s changed during host key lookup", file)
		}
		marker, _, key, _, _, err := ssh.ParseKnownHosts(lines[known.Line-1])
		if err != nil || !bytes.Equal(key.Marshal(), known.Key.Marshal()) {
			return nil, fmt.Errorf("%s changed during host key lookup", file)
		}
		if marker == "cert-authority" {
			// The CA's signing key type does not determine the host certificate's
			// key type. Any supported certificate algorithm can use this CA.
			certAuthority = true
			continue
		}
		allowed[key.Type()] = true
		switch key.Type() {
		case ssh.KeyAlgoRSA:
			// RSA key files use ssh-rsa for both SHA-1 and SHA-2 signatures.
			allowed[ssh.KeyAlgoRSASHA256] = true
			allowed[ssh.KeyAlgoRSASHA512] = true
		case ssh.CertAlgoRSAv01:
			allowed[ssh.CertAlgoRSASHA256v01] = true
			allowed[ssh.CertAlgoRSASHA512v01] = true
		}
	}

	// Prefer modern algorithms without removing legacy support offered by
	// SSH's existing defaults. FIPS mode disables those legacy algorithms.
	supported := ssh.SupportedAlgorithms().HostKeys
	if !fips140.Enabled() {
		supported = append(supported, ssh.InsecureAlgorithms().HostKeys...)
	}
	algorithms := make([]string, 0, len(supported))
	for _, algorithm := range supported {
		if allowed[algorithm] || certAuthority && strings.Contains(algorithm, "-cert-") {
			algorithms = append(algorithms, algorithm)
		}
	}
	if len(algorithms) == 0 {
		return nil, fmt.Errorf("no supported host key algorithms for %s in %s", address, file)
	}
	return algorithms, nil
}

// hostKeyLookup cannot match a parsed SSH key. A mismatch exposes the matching
// known-host entries through knownhosts.KeyError.Want without changing the file.
type hostKeyLookup struct{}

func (hostKeyLookup) Type() string    { return "host-key-lookup" }
func (hostKeyLookup) Marshal() []byte { return []byte("host key lookup") }
func (hostKeyLookup) Verify([]byte, *ssh.Signature) error {
	return errors.New("host key lookup cannot verify signatures")
}
