package lxc

import (
	"testing"
	"time"
)

func TestConfig_Prepare(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		wantErr  bool
		errField string // optional: check for specific error field
	}{
		{
			name: "valid config with password",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHPort:     22,
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				Storage:     "local-lvm",
				Memory:      1024,
				Cores:       2,
				RootfsSize:  "2",
				SSHTimeout:  "5m",
			},
			wantErr: false,
		},
		{
			name: "valid config with ssh key",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHPort:     22,
				SSHUser:     "root@pam",
				SSHKeyPath:  "/path/to/key",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				Storage:     "local-lvm",
				Memory:      1024,
				Cores:       2,
				RootfsSize:  "2",
				SSHTimeout:  "5m",
			},
			wantErr: false,
		},
		{
			name: "valid config with defaults applied",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				RootfsSize:  "2",
			},
			wantErr: false,
		},
		{
			name: "missing ssh host",
			config: &Config{
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				SSHPort:     22,
				RootfsSize:  "2",
			},
			wantErr: true,
		},
		{
			name: "missing ssh user",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				SSHPort:     22,
				RootfsSize:  "2",
			},
			wantErr: true,
		},
		{
			name: "missing auth credentials",
			config: &Config{
				SSHHost:    "192.168.1.100",
				SSHUser:    "root@pam",
				Template:   "local:vztmpl/ubuntu-22.04.tar.gz",
				SSHPort:    22,
				RootfsSize: "2",
			},
			wantErr: true,
		},
		{
			name: "missing template",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				SSHPort:     22,
				RootfsSize:  "2",
			},
			wantErr: true,
		},
		{
			name: "invalid port - negative",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHPort:     -1,
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				RootfsSize:  "2",
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHPort:     70000,
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				RootfsSize:  "2",
			},
			wantErr: true,
		},
		{
			name: "invalid memory - negative",

			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				Memory:      -1,
				SSHPort:     22,
				RootfsSize:  "2",
			},
			wantErr: true,
		},
		{
			name: "invalid cores - negative",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				Cores:       -1,
				SSHPort:     22,
				RootfsSize:  "2",
			},
			wantErr: true,
		},
		{
			name: "invalid rootfs size - not a number",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				RootfsSize:  "invalid",
				SSHPort:     22,
			},
			wantErr: true,
		},
		{
			name: "invalid ssh timeout - bad format",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				SSHTimeout:  "invalid",
				SSHPort:     22,
				RootfsSize:  "2",
			},
			wantErr: true,
		},
		{
			name: "empty ssh timeout gets default",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				SSHTimeout:  "",
				SSHPort:     22,
				RootfsSize:  "2",
			},
			wantErr: false,
		},
		{
			name: "valid config with unprivileged false",
			config: &Config{
				SSHHost:       "192.168.1.100",
				SSHPort:       22,
				SSHUser:       "root@pam",
				SSHPassword:   "secret",
				Template:      "local:vztmpl/ubuntu-22.04.tar.gz",
				Storage:       "local-lvm",
				Memory:        1024,
				Cores:         2,
				RootfsSize:    "2",
				SSHTimeout:    "5m",
				Unprivileged:  false,
			},
			wantErr: false,
		},
		{
			name: "valid config with custom features",
			config: &Config{
				SSHHost:     "192.168.1.100",
				SSHPort:     22,
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				Storage:     "local-lvm",
				Memory:      1024,
				Cores:       2,
				RootfsSize:  "2",
				SSHTimeout:  "5m",
				Features:    "nesting=1,keyctl=1",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := tt.config.Prepare(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Prepare() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_Prepare_Defaults(t *testing.T) {
	config := &Config{
		SSHHost:     "192.168.1.100",
		SSHUser:     "root@pam",
		SSHPassword: "secret",
		Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
		Unprivileged: true, // Set explicitly since mapstructure default not applied when creating directly
	}

	_, _, err := config.Prepare(nil)
	if err != nil {
		t.Fatalf("Config.Prepare() unexpected error = %v", err)
	}

	// Check defaults
	if config.SSHPort != 22 {
		t.Errorf("Expected default SSHPort 22, got %d", config.SSHPort)
	}
	if config.Storage != "local-lvm" {
		t.Errorf("Expected default Storage 'local-lvm', got %s", config.Storage)
	}
	if config.Bridge != "vmbr0" {
		t.Errorf("Expected default Bridge 'vmbr0', got %s", config.Bridge)
	}
	if config.Memory != 2048 {
		t.Errorf("Expected default Memory 2048, got %d", config.Memory)
	}
	if config.RootfsSize != "8" {
		t.Errorf("Expected default RootfsSize '8', got %s", config.RootfsSize)
	}
	if config.Cores != 2 {
		t.Errorf("Expected default Cores 2, got %d", config.Cores)
	}
	if config.RootPassword != "changeme" {
		t.Errorf("Expected default RootPassword 'changeme', got %s", config.RootPassword)
	}
	if config.Features != "nesting=1" {
		t.Errorf("Expected default Features 'nesting=1', got %s", config.Features)
	}
	if config.BackupDir != "/var/lib/vz/template/cache" {
		t.Errorf("Expected default BackupDir '/var/lib/vz/template/cache', got %s", config.BackupDir)
	}
	if config.SSHTimeout != "5m" {
		t.Errorf("Expected default SSHTimeout '5m', got %s", config.SSHTimeout)
	}
}

func TestConfig_Prepare_UnprivilegedDefault(t *testing.T) {
	// Test that Unprivileged can be set to true when creating Config directly
	config := &Config{
		SSHHost:     "192.168.1.100",
		SSHUser:     "root@pam",
		SSHPassword: "secret",
		Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
		RootfsSize:  "2",
		Unprivileged: true,
	}

	_, _, err := config.Prepare(nil)
	if err != nil {
		t.Fatalf("Config.Prepare() unexpected error = %v", err)
	}

	if config.Unprivileged != true {
		t.Errorf("Expected Unprivileged true, got %v", config.Unprivileged)
	}
}

func TestConfig_Prepare_UnprivilegedFalse(t *testing.T) {
	// Test that Unprivileged can be set to false using raw config
	config := &Config{}
	raw := map[string]interface{}{
		"ssh_host":      "192.168.1.100",
		"ssh_user":      "root@pam",
		"ssh_password":  "secret",
		"template":      "local:vztmpl/ubuntu-22.04.tar.gz",
		"rootfs_size":   "2",
		"unprivileged":  false,
	}

	_, _, err := config.Prepare(raw)
	if err != nil {
		t.Fatalf("Config.Prepare() unexpected error = %v", err)
	}

	if config.Unprivileged != false {
		t.Errorf("Expected Unprivileged false, got %v", config.Unprivileged)
	}
}

func TestConfig_Prepare_EmptySSHTimeout(t *testing.T) {
	config := &Config{
		SSHHost:     "192.168.1.100",
		SSHUser:     "root@pam",
		SSHPassword: "secret",
		Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
		SSHTimeout:  "",
		RootfsSize:  "2",
	}

	_, _, err := config.Prepare(nil)
	if err != nil {
		t.Fatalf("Config.Prepare() unexpected error = %v", err)
	}

	if config.SSHTimeout != "5m" {
		t.Errorf("Expected SSHTimeout to be set to default '5m', got %s", config.SSHTimeout)
	}
}

func TestConfig_Prepare_RootfsSizeFormats(t *testing.T) {
	tests := []struct {
		name       string
		rootfsSize string
		wantErr    bool
	}{
		{
			name:       "valid single digit",
			rootfsSize: "2",
			wantErr:    false,
		},
		{
			name:       "valid multi digit",
			rootfsSize: "100",
			wantErr:    false,
		},
		{
			name:       "valid with G unit",
			rootfsSize: "2G",
			wantErr:    false,
		},
		{
			name:       "valid with GB unit",
			rootfsSize: "2GB",
			wantErr:    false,
		},
		{
			name:       "valid with MB unit",
			rootfsSize: "2048MB",
			wantErr:    false,
		},
		{
			name:       "valid decimal with GB",
			rootfsSize: "1.5GB",
			wantErr:    false,
		},
		{
			name:       "invalid letters only",
			rootfsSize: "abc",
			wantErr:    true,
		},
		{
			name:       "empty string gets default",
			rootfsSize: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				SSHPort:     22,
				RootfsSize:  tt.rootfsSize,
			}
			_, _, err := config.Prepare(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Prepare() with rootfsSize %q error = %v, wantErr %v", tt.rootfsSize, err, tt.wantErr)
			}
		})
	}
}

func TestParseSizeToGB(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "plain number",
			input:    "2",
			expected: "2",
			wantErr:  false,
		},
		{
			name:     "with G unit",
			input:    "2G",
			expected: "2",
			wantErr:  false,
		},
		{
			name:     "with GB unit",
			input:    "2GB",
			expected: "2",
			wantErr:  false,
		},
		{
			name:     "with MB unit (2048MB = 2GB)",
			input:    "2048MB",
			expected: "2",
			wantErr:  false,
		},
		{
			name:     "with M unit",
			input:    "2048M",
			expected: "2",
			wantErr:  false,
		},
		{
			name:     "decimal with GB",
			input:    "1.5GB",
			expected: "1.5",
			wantErr:  false,
		},
		{
			name:     "with TB unit",
			input:    "1TB",
			expected: "1024",
			wantErr:  false,
		},
		{
			name:     "invalid letters only",
			input:    "abc",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSizeToGB(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSizeToGB(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parseSizeToGB(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestConfig_Prepare_SSHTimeoutFormats(t *testing.T) {
	tests := []struct {
		name       string
		sshTimeout string
		wantErr    bool
	}{
		{
			name:       "valid minutes",
			sshTimeout: "5m",
			wantErr:    false,
		},
		{
			name:       "valid seconds",
			sshTimeout: "30s",
			wantErr:    false,
		},
		{
			name:       "valid hours",
			sshTimeout: "1h",
			wantErr:    false,
		},
		{
			name:       "valid complex",
			sshTimeout: "1h30m",
			wantErr:    false,
		},
		{
			name:       "invalid format",
			sshTimeout: "invalid",
			wantErr:    true,
		},
		{
			name:       "empty gets default",
			sshTimeout: "",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				SSHHost:     "192.168.1.100",
				SSHUser:     "root@pam",
				SSHPassword: "secret",
				Template:    "local:vztmpl/ubuntu-22.04.tar.gz",
				SSHPort:     22,
				RootfsSize:  "2",
				SSHTimeout:  tt.sshTimeout,
			}
			_, _, err := config.Prepare(nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Prepare() with sshTimeout %q error = %v, wantErr %v", tt.sshTimeout, err, tt.wantErr)
			}

			// Verify timeout can be parsed as duration if no error
			if err == nil && tt.sshTimeout != "" {
				if _, err := time.ParseDuration(config.SSHTimeout); err != nil {
					t.Errorf("SSHTimeout %q cannot be parsed as duration: %v", config.SSHTimeout, err)
				}
			}
		})
	}
}

func TestConfig_Prepare_WithRawConfig(t *testing.T) {
	// Test that Prepare works with raw config map via decode
	config := &Config{}
	raw := map[string]interface{}{
		"ssh_host":      "192.168.1.100",
		"ssh_port":      2222,
		"ssh_user":      "root@pam",
		"ssh_password":  "secret",
		"template":      "local:vztmpl/ubuntu-22.04.tar.gz",
		"storage":       "local",
		"bridge":        "vmbr1",
		"memory":        2048,
		"cores":         4,
		"rootfs_size":   "4",
		"ssh_timeout":   "10m",
		"unprivileged":  false,
		"features":      "nesting=1,keyctl=1",
		"root_password": "test123",
		"backup_name":   "my-template",
		"backup_dir":    "/tmp/backups",
		"ctid":          "200",
		"lxc_config":    "lxc.apparmor.profile: unconfined",
	}

	_, _, err := config.Prepare(raw)
	if err != nil {
		t.Fatalf("Config.Prepare() with raw config error = %v", err)
	}

	// Verify values were decoded correctly
	if config.SSHHost != "192.168.1.100" {
		t.Errorf("Expected SSHHost '192.168.1.100', got %s", config.SSHHost)
	}
	if config.SSHPort != 2222 {
		t.Errorf("Expected SSHPort 2222, got %d", config.SSHPort)
	}
	if config.Memory != 2048 {
		t.Errorf("Expected Memory 2048, got %d", config.Memory)
	}
	if config.Cores != 4 {
		t.Errorf("Expected Cores 4, got %d", config.Cores)
	}
	if config.Unprivileged != false {
		t.Errorf("Expected Unprivileged false, got %v", config.Unprivileged)
	}
	if config.CTID != "200" {
		t.Errorf("Expected CTID '200', got %s", config.CTID)
	}
	if config.LXCConfig != "lxc.apparmor.profile: unconfined" {
		t.Errorf("Expected LXCConfig 'lxc.apparmor.profile: unconfined', got %s", config.LXCConfig)
	}
}
