//go:build integration

package lxc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

// dialProxmoxSSH opens a direct SSH connection to the Proxmox host,
// independent of the Builder's internal communicator, so tests can run
// their own cleanup commands after Builder.Run returns.
func dialProxmoxSSH(host string, port int, user, password, keyPath string) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	if password != "" {
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(password)}
	} else {
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH key: %w", err)
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	return ssh.Dial("tcp", addr, sshConfig)
}

// runCommandsOnHost opens one SSH connection to the Proxmox host and runs
// each command in turn, best-effort: failures are logged, not fatal, since
// the test itself has already passed or failed by the time cleanup runs.
func runCommandsOnHost(t *testing.T, host string, port int, user, password, keyPath string, cmds ...string) {
	t.Helper()
	client, err := dialProxmoxSSH(host, port, user, password, keyPath)
	if err != nil {
		t.Logf("cleanup: failed to connect to Proxmox host: %v", err)
		return
	}
	defer client.Close()

	for _, cmd := range cmds {
		session, err := client.NewSession()
		if err != nil {
			t.Logf("cleanup: failed to open session for %q: %v", cmd, err)
			continue
		}
		if err := session.Run(cmd); err != nil {
			t.Logf("cleanup: %q failed: %v", cmd, err)
		}
		session.Close()
	}
}

// destroyContainerOnHost best-effort stops and destroys a container/CT
// template directly via SSH, for tests to clean up after themselves.
func destroyContainerOnHost(t *testing.T, host string, port int, user, password, keyPath, ctid string) {
	t.Helper()
	t.Logf("cleanup: destroying CTID %s", ctid)
	runCommandsOnHost(t, host, port, user, password, keyPath,
		fmt.Sprintf("pct stop %s", ctid),
		fmt.Sprintf("pct destroy %s --purge", ctid),
	)
}

// removeFileOnHost best-effort deletes a file directly via SSH, for tests
// to clean up backup artifacts (e.g. vzdump .tar.gz files) they created.
func removeFileOnHost(t *testing.T, host string, port int, user, password, keyPath, path string) {
	t.Helper()
	t.Logf("cleanup: removing %s", path)
	runCommandsOnHost(t, host, port, user, password, keyPath, fmt.Sprintf("rm -f %s", path))
}

// TestIntegration_Connect tests the full connection flow to a Proxmox host.
// Required environment variables:
//   - PROXMOX_HOST: Proxmox host address
//   - PROXMOX_PORT: SSH port (default: 22)
//   - PROXMOX_USER: SSH username (e.g., root@pam)
//   - PROXMOX_PASSWORD: SSH password
//   - PROXMOX_KEY_PATH: Path to SSH key (alternative to password)
func TestIntegration_Connect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := os.Getenv("PROXMOX_HOST")
	if host == "" {
		t.Skip("PROXMOX_HOST not set")
	}

	port := 22
	if portStr := os.Getenv("PROXMOX_PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			t.Fatalf("Invalid PROXMOX_PORT: %v", err)
		}
		port = p
	}

	user := os.Getenv("PROXMOX_USER")
	if user == "" {
		t.Skip("PROXMOX_USER not set")
	}

	password := os.Getenv("PROXMOX_PASSWORD")
	keyPath := os.Getenv("PROXMOX_KEY_PATH")

	if password == "" && keyPath == "" {
		t.Skip("Either PROXMOX_PASSWORD or PROXMOX_KEY_PATH must be set")
	}

	client, err := dialProxmoxSSH(host, port, user, password, keyPath)
	if err != nil {
		t.Fatalf("Failed to connect to Proxmox host: %v", err)
	}
	defer client.Close()

	t.Log("Successfully connected to Proxmox host")
}

// TestIntegration_FullBuild tests a full build process.
// Required environment variables: See TestIntegration_Connect, plus:
//   - PROXMOX_TEMPLATE: Template to use (e.g., local:vztmpl/ubuntu-22.04.tar.gz)
//   - PROXMOX_STORAGE: Storage for container (default: local-lvm)
//
// Optional:
//   - PROXMOX_CTID: Pin a specific CTID instead of auto-assigning one
//     (makes manual cleanup on your cluster predictable)
//
// Optional network environment variables (exercise the vlan/static-IP
// support added for GitHub issue #1):
//   - PROXMOX_BRIDGE: Network bridge (default: vmbr0)
//   - PROXMOX_VLAN: VLAN tag for the bridge, e.g. "100"
//   - PROXMOX_NETWORK_IP: "dhcp", "manual", or a static IP in CIDR notation
//   - PROXMOX_GATEWAY: Gateway IP, used when PROXMOX_NETWORK_IP is static
//   - PROXMOX_BACKUP_PIGZ: pigz thread count for vzdump (GitHub issue #4).
//     Default is 1 (auto, half of cores) if unset; ">1" pins a thread
//     count; "-1" explicitly disables pigz in favor of plain gzip.
func TestIntegration_FullBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := os.Getenv("PROXMOX_HOST")
	if host == "" {
		t.Skip("PROXMOX_HOST not set")
	}

	port := 22
	if portStr := os.Getenv("PROXMOX_PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			t.Fatalf("Invalid PROXMOX_PORT: %v", err)
		}
		port = p
	}
	user := os.Getenv("PROXMOX_USER")
	password := os.Getenv("PROXMOX_PASSWORD")
	keyPath := os.Getenv("PROXMOX_KEY_PATH")

	template := os.Getenv("PROXMOX_TEMPLATE")
	if template == "" {
		t.Skip("PROXMOX_TEMPLATE not set")
	}

	// This creates, provisions, backs up, and destroys a real container on
	// the target Proxmox host. The container itself is destroyed by the
	// plugin as part of the normal vzdump flow, but the resulting backup
	// .tar.gz is the deliverable artifact and is intentionally left in
	// place — so this test removes it itself via t.Cleanup once done, to
	// avoid leaving real files behind on your cluster across repeated
	// local runs.
	config := map[string]interface{}{
		"ssh_host":     host,
		"ssh_user":     user,
		"ssh_password": password,
		"ssh_key_path": keyPath,
		"template":     template,
		"storage":      os.Getenv("PROXMOX_STORAGE"),
		"backup_dir":   "/var/lib/vz/template/cache",
		"rootfs_size":  "2",
	}

	if ctid := os.Getenv("PROXMOX_CTID"); ctid != "" {
		config["ctid"] = ctid
	}
	if bridge := os.Getenv("PROXMOX_BRIDGE"); bridge != "" {
		config["bridge"] = bridge
	}
	if vlan := os.Getenv("PROXMOX_VLAN"); vlan != "" {
		tag, err := strconv.Atoi(vlan)
		if err != nil {
			t.Fatalf("Invalid PROXMOX_VLAN: %v", err)
		}
		config["vlan"] = tag
	}
	if networkIP := os.Getenv("PROXMOX_NETWORK_IP"); networkIP != "" {
		config["network_ip"] = networkIP
	}
	if gateway := os.Getenv("PROXMOX_GATEWAY"); gateway != "" {
		config["gateway"] = gateway
	}
	if pigz := os.Getenv("PROXMOX_BACKUP_PIGZ"); pigz != "" {
		threads, err := strconv.Atoi(pigz)
		if err != nil {
			t.Fatalf("Invalid PROXMOX_BACKUP_PIGZ: %v", err)
		}
		config["backup_pigz"] = threads
	}

	b := &Builder{}
	_, warnings, err := b.Prepare(config)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("Warnings: %v", warnings)
	}

	ui := &testUi{}
	// A real "packer build" always supplies a non-nil Hook (even with no
	// provisioners configured); StepProvision calls methods on it
	// unconditionally, so passing nil here would panic.
	artifact, err := b.Run(context.Background(), ui, &packersdk.DispatchHook{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if a, ok := artifact.(*Artifact); ok && a.BackupPath != "" {
		t.Cleanup(func() {
			removeFileOnHost(t, host, port, user, password, keyPath, a.BackupPath)
		})
	}

	t.Logf("Artifact: %v", artifact)
}

// TestIntegration_FullBuild_TemplateMethod tests a full build process using
// backup_method = "template" (GitHub issue #3): instead of a vzdump backup,
// the container is stopped and converted in place into a Proxmox CT
// template via `pct template`. Unlike TestIntegration_FullBuild, the
// plugin intentionally does NOT destroy it (that's the whole point of
// backup_method=template) — so this test destroys it itself via
// t.Cleanup once the test finishes, to avoid leaving real containers
// behind on your cluster across repeated local runs.
//
// Required environment variables: See TestIntegration_FullBuild.
func TestIntegration_FullBuild_TemplateMethod(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := os.Getenv("PROXMOX_HOST")
	if host == "" {
		t.Skip("PROXMOX_HOST not set")
	}

	port := 22
	if portStr := os.Getenv("PROXMOX_PORT"); portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err != nil {
			t.Fatalf("Invalid PROXMOX_PORT: %v", err)
		}
		port = p
	}
	user := os.Getenv("PROXMOX_USER")
	password := os.Getenv("PROXMOX_PASSWORD")
	keyPath := os.Getenv("PROXMOX_KEY_PATH")

	template := os.Getenv("PROXMOX_TEMPLATE")
	if template == "" {
		t.Skip("PROXMOX_TEMPLATE not set")
	}

	// This creates and provisions a real container on the target Proxmox
	// host and converts it into a CT template; see the cleanup note above.
	config := map[string]interface{}{
		"ssh_host":      host,
		"ssh_user":      os.Getenv("PROXMOX_USER"),
		"ssh_password":  os.Getenv("PROXMOX_PASSWORD"),
		"ssh_key_path":  os.Getenv("PROXMOX_KEY_PATH"),
		"template":      template,
		"storage":       os.Getenv("PROXMOX_STORAGE"),
		"rootfs_size":   "2",
		"backup_method": "template",
	}
	if ctid := os.Getenv("PROXMOX_CTID"); ctid != "" {
		config["ctid"] = ctid
	}

	b := &Builder{}
	_, warnings, err := b.Prepare(config)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("Warnings: %v", warnings)
	}

	ui := &testUi{}
	// See the note in TestIntegration_FullBuild about why nil can't be
	// passed here.
	rawArtifact, err := b.Run(context.Background(), ui, &packersdk.DispatchHook{})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	artifact, ok := rawArtifact.(*Artifact)
	if !ok {
		t.Fatalf("Expected *Artifact, got %T", rawArtifact)
	}
	if artifact.CTID != "" {
		// Runs even if the assertions below fail, so the template never
		// lingers on the cluster just because an assertion tripped.
		t.Cleanup(func() {
			destroyContainerOnHost(t, host, port, user, password, keyPath, artifact.CTID)
		})
	}

	if artifact.Method != "template" {
		t.Errorf("Expected Method 'template', got %q", artifact.Method)
	}
	if artifact.CTID == "" {
		t.Error("Expected a non-empty CTID for the resulting CT template")
	}

	t.Logf("Artifact: %v (CTID: %s)", artifact, artifact.CTID)
}
