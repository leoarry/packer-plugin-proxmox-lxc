//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package lxc

import (
	"fmt"
	
	"strconv"
	"strings"
	"time"

	common "github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)
// sizeUnitMultiplier maps size units to their multiplier for converting to GB.
// An empty unit means GB (no conversion needed).
var sizeUnitMultiplier = map[string]float64{
	"":     1,             // GB (default)
	"G":     1,             // GB
	"GB":    1,             // GB
	"M":     1.0 / 1024.0, // MB to GB
	"MB":    1.0 / 1024.0, // MB to GB
	"T":     1024,          // TB to GB
	"TB":    1024,          // TB to GB
}

// parseSizeToGB parses a size string like "2", "2GB", "2048MB" and returns
// the numeric value in GB as a string (for use in pct create --rootfs).
func parseSizeToGB(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("size cannot be empty")
	}

	// Find where the numeric part ends and the unit begins
	numEnd := 0
	for i, r := range input {
		if (r >= '0' && r <= '9') || r == '.' {
			numEnd = i + 1
		} else {
			break
		}
	}

	if numEnd == 0 {
		return "", fmt.Errorf("invalid size format: %s (expected number with optional unit like 2, 2GB, 2048MB)", input)
	}

	numStr := input[:numEnd]
	unit := strings.TrimSpace(strings.ToUpper(input[numEnd:]))

	// Parse the number
	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return "", fmt.Errorf("invalid size number: %s", numStr)
	}

	// Get the multiplier for the unit
	multiplier, ok := sizeUnitMultiplier[unit]
	if !ok {
		return "", fmt.Errorf("unknown size unit: %s (supported: GB, MB, TB, G, M, T)", unit)
	}

	// Convert to GB
	gbValue := value * multiplier

	// Return as string, removing trailing zeros for clean output
	result := strconv.FormatFloat(gbValue, 'f', -1, 64)
	return result, nil
}


// Config defines the configuration for the Proxmox LXC builder.
type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	// SSH connection settings.
	SSHHost       string `mapstructure:"ssh_host" required:"true"`
	SSHPort       int    `mapstructure:"ssh_port" default:"22"`
	SSHUser       string `mapstructure:"ssh_user" required:"true"`
	SSHPassword   string `mapstructure:"ssh_password"`
	SSHKeyPath    string `mapstructure:"ssh_key_path"`

	// LXC container settings.
	Template     string `mapstructure:"template" required:"true"`
	Storage      string `mapstructure:"storage" default:"local-lvm"`
	Bridge       string `mapstructure:"bridge" default:"vmbr0"`
	Memory       int    `mapstructure:"memory" default:"2048"`
	Cores        int    `mapstructure:"cores" default:"2"`
	RootPassword string `mapstructure:"root_password" default:"changeme"`
	Unprivileged bool   `mapstructure:"unprivileged" default:"true"`
	Features     string `mapstructure:"features" default:"nesting=1"`
	RootfsSize   string `mapstructure:"rootfs_size" default:"8"`

	// Build settings.
	BackupName   string `mapstructure:"backup_name"`
	BackupDir    string `mapstructure:"backup_dir" default:"/var/lib/vz/template/cache"`
	CTID         string `mapstructure:"ctid"`
	LXCConfig    string `mapstructure:"lxc_config"`

	// SSH timeout for provisioning.
	SSHTimeout string `mapstructure:"ssh_timeout" default:"5m"`
}

// applyDefaults sets default values for Config fields.
// Note: Unprivileged default is handled by mapstructure during Decode.
// For Config created directly (not from raw config), Unprivileged will
// be false (Go default) unless explicitly set.
func (c *Config) applyDefaults() {
	if c.SSHPort == 0 {
		c.SSHPort = 22
	}
	if c.Storage == "" {
		c.Storage = "local-lvm"
	}
	if c.Bridge == "" {
		c.Bridge = "vmbr0"
	}
	if c.Memory == 0 {
		c.Memory = 2048
	}
	if c.Cores == 0 {
		c.Cores = 2
	}
	if c.RootPassword == "" {
		c.RootPassword = "changeme"
	}
	if c.Features == "" {
		c.Features = "nesting=1"
	}
	if c.RootfsSize == "" {
		c.RootfsSize = "8"
	}
	if c.BackupDir == "" {
		c.BackupDir = "/var/lib/vz/template/cache"
	}
	if c.SSHTimeout == "" {
		c.SSHTimeout = "5m"
	}
}

// Prepare validates the configuration.
func (c *Config) Prepare(raws ...interface{}) ([]string, []string, error) {
	var warnings []string

	err := config.Decode(c, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &interpolate.Context{},
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{
				"ssh_password",
				"ssh_key_path",
			},
		},
	}, raws...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Apply defaults after decoding
	c.applyDefaults()

	// Validate required fields.
	if c.SSHHost == "" {
		return nil, nil, fmt.Errorf("ssh_host is required")
	}
	if c.SSHUser == "" {
		return nil, nil, fmt.Errorf("ssh_user is required")
	}
	if c.SSHPassword == "" && c.SSHKeyPath == "" {
		return nil, nil, fmt.Errorf("either ssh_password or ssh_key_path is required")
	}
	if c.Template == "" {
		return nil, nil, fmt.Errorf("template is required")
	}

	// Validate numeric fields.
	if c.SSHPort <= 0 || c.SSHPort > 65535 {
		return nil, nil, fmt.Errorf("ssh_port must be between 1 and 65535")
	}
	if c.Memory <= 0 {
		return nil, nil, fmt.Errorf("memory must be greater than 0")
	}
	if c.Cores <= 0 {
		return nil, nil, fmt.Errorf("cores must be greater than 0")
	}

	// Validate rootfs size (accepts plain numbers or numbers with units like 2GB, 2048MB).
	parsedSize, err := parseSizeToGB(c.RootfsSize)
	if err != nil {
		return nil, nil, fmt.Errorf("rootfs_size: %w", err)
	}
	// Update RootfsSize to the parsed value (number in GB) for use in pct create.
	c.RootfsSize = parsedSize

	// Validate SSH timeout.
	if _, err := time.ParseDuration(c.SSHTimeout); err != nil {
		return nil, nil, fmt.Errorf("invalid ssh_timeout format: %w", err)
	}

	return warnings, nil, nil
}
