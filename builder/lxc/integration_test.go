//go:build integration

package lxc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

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
			t.Fatalf("Failed to read SSH key: %v", err)
		}
		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			t.Fatalf("Failed to parse SSH key: %v", err)
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
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
// Optional network environment variables (exercise the vlan/static-IP
// support added for GitHub issue #1):
//   - PROXMOX_BRIDGE: Network bridge (default: vmbr0)
//   - PROXMOX_VLAN: VLAN tag for the bridge, e.g. "100"
//   - PROXMOX_NETWORK_IP: "dhcp", "manual", or a static IP in CIDR notation
//   - PROXMOX_GATEWAY: Gateway IP, used when PROXMOX_NETWORK_IP is static
func TestIntegration_FullBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := os.Getenv("PROXMOX_HOST")
	if host == "" {
		t.Skip("PROXMOX_HOST not set")
	}

	template := os.Getenv("PROXMOX_TEMPLATE")
	if template == "" {
		t.Skip("PROXMOX_TEMPLATE not set")
	}

	// This is a skeleton test - uncomment when ready to run
	t.Skip("Full build integration test not yet enabled")

	config := map[string]interface{}{
		"ssh_host":     host,
		"ssh_user":     os.Getenv("PROXMOX_USER"),
		"ssh_password": os.Getenv("PROXMOX_PASSWORD"),
		"ssh_key_path": os.Getenv("PROXMOX_KEY_PATH"),
		"template":     template,
		"storage":      os.Getenv("PROXMOX_STORAGE"),
		"backup_dir":   "/var/lib/vz/template/cache",
		"rootfs_size":  "2",
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

	b := &Builder{}
	_, warnings, err := b.Prepare(config)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("Warnings: %v", warnings)
	}

	ui := &testUi{}
	artifact, err := b.Run(context.Background(), ui, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	t.Logf("Artifact: %v", artifact)
}

// TestIntegration_FullBuild_TemplateMethod tests a full build process using
// backup_method = "template" (GitHub issue #3): instead of a vzdump backup,
// the container is stopped and converted in place into a Proxmox CT
// template via `pct template`, and is left on the Proxmox host rather than
// destroyed.
//
// Required environment variables: See TestIntegration_FullBuild.
// Note: this test does NOT destroy the resulting container/template.
// Clean it up manually on the Proxmox host (`pct destroy <ctid>`) between runs.
func TestIntegration_FullBuild_TemplateMethod(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	host := os.Getenv("PROXMOX_HOST")
	if host == "" {
		t.Skip("PROXMOX_HOST not set")
	}

	template := os.Getenv("PROXMOX_TEMPLATE")
	if template == "" {
		t.Skip("PROXMOX_TEMPLATE not set")
	}

	// This is a skeleton test - uncomment when ready to run
	t.Skip("Full build integration test not yet enabled")

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

	b := &Builder{}
	_, warnings, err := b.Prepare(config)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if len(warnings) > 0 {
		t.Logf("Warnings: %v", warnings)
	}

	ui := &testUi{}
	rawArtifact, err := b.Run(context.Background(), ui, nil)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	artifact, ok := rawArtifact.(*Artifact)
	if !ok {
		t.Fatalf("Expected *Artifact, got %T", rawArtifact)
	}
	if artifact.Method != "template" {
		t.Errorf("Expected Method 'template', got %q", artifact.Method)
	}
	if artifact.CTID == "" {
		t.Error("Expected a non-empty CTID for the resulting CT template")
	}

	t.Logf("Artifact: %v (CTID: %s)", artifact, artifact.CTID)
}
