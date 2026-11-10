package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// decodeHCL decodes an HCL source block body the way Packer does: parse it,
// decode it against the generated HCL2 spec, and hand the resulting cty.Value
// to Config.Prepare. This exercises the hcl/go-cty versions pinned in go.mod and the
// generated config.hcl2spec.go, which map-based tests never touch.
func decodeHCL(t *testing.T, src string) (*Config, []string, error) {
	t.Helper()
	file, diags := hclparse.NewParser().ParseHCL([]byte(src), "test.pkr.hcl")
	if diags.HasErrors() {
		t.Fatalf("parse: %s", diags)
	}
	spec := hcldec.ObjectSpec((&Config{}).FlatMapstructure().HCL2Spec())
	val, diags := hcldec.Decode(file.Body, spec, nil)
	if diags.HasErrors() {
		return nil, nil, diags
	}
	c := &Config{}
	warnings, _, err := c.Prepare(val)
	return c, warnings, err
}

func TestConfig_HCLDecode(t *testing.T) {
	c, _, err := decodeHCL(t, `
ssh_host     = "10.0.0.5"
ssh_port     = 2222
ssh_user     = "root"
ssh_password = "secret"
template     = "local:vztmpl/debian-13-standard_13.6-1_amd64.tar.zst"
rootfs_size  = "2GB"
memory       = 1024
unprivileged = false
firewall     = true
backup_pigz  = 4
backup_name  = "tpl-${"x"}"
ssh_timeout  = "2m"
`)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	checks := []struct {
		field string
		got   any
		want  any
	}{
		{"ssh_host", c.SSHHost, "10.0.0.5"},
		{"ssh_port", c.SSHPort, 2222},
		{"memory", c.Memory, 1024},
		{"rootfs_size", c.RootfsSize, "2"},
		{"unprivileged", c.IsUnprivileged(), false},
		{"firewall", c.Firewall, true},
		{"backup_pigz", c.BackupPigz, 4},
		{"backup_name", c.BackupName, "tpl-x"},
		{"ssh_timeout", c.SSHTimeout, "2m"},
		// Unset attributes still get their defaults.
		{"storage", c.Storage, "local-lvm"},
		{"cores", c.Cores, 2},
	}
	for _, ck := range checks {
		if ck.got != ck.want {
			t.Errorf("%s = %v, want %v", ck.field, ck.got, ck.want)
		}
	}
}

func TestConfig_HCLDecode_TypeErrors(t *testing.T) {
	_, _, err := decodeHCL(t, `
ssh_host     = "10.0.0.5"
ssh_user     = "root"
ssh_password = "secret"
template     = "local:vztmpl/x.tar.zst"
memory       = "lots"
`)
	if err == nil {
		t.Fatal("expected an error for a non-numeric memory value")
	}
}

// TestConfig_HCL2SpecUpToDate fails when a Config field was added or renamed
// without running `make generate`: Packer would reject the attribute in an HCL
// template even though map-based tests pass.
func TestConfig_HCL2SpecUpToDate(t *testing.T) {
	spec := (&Config{}).FlatMapstructure().HCL2Spec()
	rt := reflect.TypeOf(Config{})
	for i := 0; i < rt.NumField(); i++ {
		name, _, _ := strings.Cut(rt.Field(i).Tag.Get("mapstructure"), ",")
		if name == "" {
			continue // squashed common.PackerConfig
		}
		if _, ok := spec[name]; !ok {
			t.Errorf("%s is missing from config.hcl2spec.go; run `make generate`", name)
		}
	}
}
