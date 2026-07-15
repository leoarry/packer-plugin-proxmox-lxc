# packer-plugin-proxmox-lxc

A HashiCorp Packer plugin for building LXC container templates on Proxmox VE. This plugin allows you to create reusable LXC templates from a local machine by connecting to a Proxmox host via SSH.

## Features

- Build LXC container templates from any machine with SSH access to Proxmox
- HCL2 configuration support
- Supports both password and SSH key authentication
- Runs provisioners (shell, file) inside the LXC container
- Produces `.tar.gz` LXC template artifacts ready for deployment
- Installable via `packer init`

## Installation

### Via `packer init` (Recommended)

Add the plugin to your Packer configuration:

```hcl
packer {
  required_plugins {
    proxmox-lxc = {
      version = ">= 0.1.0"
      source  = "github.com/leoarry/proxmox-lxc"
    }
  }
}
```

Then run:

```bash
packer init .
```

### Local Build

```bash
git clone https://github.com/leoarry/packer-plugin-proxmox-lxc.git
cd packer-plugin-proxmox-lxc
make build
packer plugins install --path ./packer-plugin-proxmox-lxc github.com/leoarry/proxmox-lxc
```

## Quick Start

Create a `template.pkr.hcl` file:

```hcl
packer {
  required_plugins {
    proxmox-lxc = {
      version = ">= 0.1.0"
      source  = "github.com/leoarry/proxmox-lxc"
    }
  }
}

source "proxmox-lxc" "ubuntu" {
  # SSH connection
  ssh_host = "192.168.1.100"
  ssh_user = "root"
  ssh_password = "your-password"

  # LXC template
  template     = "local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst"
  storage      = "local-lvm"
  memory       = 2048
  cores        = 2

  # Backup settings
  backup_name = "ubuntu-22.04-template"
  backup_dir  = "/var/lib/vz/template/cache"
}

build {
  sources = ["source.proxmox-lxc.ubuntu"]

  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get upgrade -y",
      "apt-get install -y curl wget vim",
    ]
  }
}
```

Run the build:

```bash
packer build template.pkr.hcl
```

## Configuration Reference

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `ssh_host` | string | Proxmox host IP or hostname |
| `ssh_user` | string | Proxmox user (e.g., `root@pam`) |
| `template` | string | LXC template to use (e.g., `local:vztmpl/ubuntu-22.04-standard.tar.zst`) |

### Authentication (One Required)

| Field | Type | Description |
|-------|------|-------------|
| `ssh_password` | string | Proxmox user password |
| `ssh_key_path` | string | Path to SSH private key |

### Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ssh_port` | int | `22` | SSH port |
| `storage` | string | `"local-lvm"` | Storage for the container |
| `bridge` | string | `"vmbr0"` | Network bridge |
| `vlan` | int | - | VLAN tag for the bridge (1-4094). Leave unset for no VLAN tagging |
| `network_ip` | string | `"dhcp"` | Network IP config: `"dhcp"`, `"manual"`, or a static IP in CIDR notation (e.g. `"192.168.1.50/24"`) |
| `gateway` | string | - | Gateway IP address, used when `network_ip` is a static IP |
| `firewall` | bool | `false` | Enable the Proxmox firewall on the container's network interface |
| `network_mtu` | int | - | MTU for the container's network interface |
| `memory` | int | `2048` | Memory in MB |
| `cores` | int | `2` | Number of CPU cores |
| `root_password` | string | `"changeme"` | Root password for the container |
| `unprivileged` | bool | `true` | Create unprivileged container |
| `features` | string | `"nesting=1"` | LXC features (e.g., `nesting=1,keyctl=1`) |
| `rootfs_size` | string | `"8"` | Root filesystem size in GB |
| `backup_method` | string | `"vzdump"` | How to finalize the build: `"vzdump"` (backup file, container destroyed) or `"template"` (converts the container itself into a Proxmox CT template via `pct template`, container kept) |
| `backup_name` | string | auto-generated | Name for the backup file. Only used when `backup_method` is `"vzdump"`. Supports HCL2 template functions like `timestamp()` and `formatdate()` for dynamic naming, e.g., `"my-template_${formatdate("2006-01-02_15-04", timestamp())}_debian_12.7-1_amd64"` |
| `backup_dir` | string | `"/var/lib/vz/template/cache"` | Backup destination directory. Only used when `backup_method` is `"vzdump"` |
| `backup_pigz` | int | `1` | pigz thread count for faster vzdump compression (`1` = auto, half of cores; `>1` = that many threads; `-1` = explicitly disable, plain gzip). Falls back to plain gzip with a warning if `pigz` isn't installed on the host, instead of failing. Only used when `backup_method` is `"vzdump"` |
| `ctid` | string | auto-assigned | Specific CTID to use (reused if it already exists). If unset, an auto-assigned CTID that collides with a concurrent build on the same host is automatically retried with a fresh CTID (up to 5 times) instead of failing or reusing the other build's container |
| `lxc_config` | string | - | Additional LXC config to merge |
| `ssh_timeout` | string | `"5m"` | SSH connection timeout |

## Examples

See the `example/` directory for complete examples:

- `example/basic.pkr.hcl` - Basic Ubuntu template build
- `example/advanced.pkr.hcl` - Advanced setup with SSH key auth and k3s installation
- `example/ct-template.pkr.hcl` - Produces a Proxmox CT template (`backup_method = "template"`) instead of a vzdump backup

## How It Works

The plugin automates the following steps on your Proxmox host:

1. Connects to Proxmox via SSH
2. Gets the next available CTID (or uses configured one)
3. Creates an LXC container from the specified template
4. Merges custom LXC config (if provided)
5. Starts the container
6. Runs provisioners inside the container
7. Finalizes the container using the configured `backup_method`:
   - `vzdump` (default): creates a backup via `vzdump`, then stops and destroys the container (leaving the backup)
   - `template`: stops the container and converts it into a Proxmox CT template via `pct template` (the container is kept, not destroyed)

## Development

### Prerequisites

- Go 1.21 or later
- Make
- Packer

### Building

```bash
make build
```

### Testing

```bash
make test
```

### Integration Testing

The unit test suite above uses mocks and needs no real infrastructure. A
separate integration suite (`builder/lxc/integration_test.go`, build tag
`integration`) exercises the plugin against a **real** Proxmox host —
connecting over SSH, creating and provisioning an actual container, and
either backing it up (`vzdump`) or converting it into a CT template.

Two safe, no-infrastructure-needed checks run as part of normal
development (and CI):

```bash
make vet-integration    # compile-only check, catches build errors early
make test-integration   # runs the integration tests, but they self-skip
                         # without a real PROXMOX_HOST
```

To actually run them against your own Proxmox cluster, pass connection
details as variables to `test-integration-full`:

```bash
make test-integration-full \
  PROXMOX_HOST=10.0.0.5 PROXMOX_USER=root PROXMOX_PASSWORD=secret \
  PROXMOX_TEMPLATE=local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst
```

Notes:

- `ssh_user` must be a real Linux login (e.g. `root`), **not** Proxmox's
  `user@realm` API/GUI format (`root@pam`) — this plugin connects over
  plain SSH, not the Proxmox API.
- Use `PROXMOX_KEY_PATH` instead of `PROXMOX_PASSWORD` for key-based auth.
- `RUN` narrows which tests run (default: all `TestIntegration*`), e.g.
  `RUN=TestIntegration_Connect` to just check connectivity, or
  `RUN=TestIntegration_FullBuild_TemplateMethod` for just the CT template
  path. `TIMEOUT` bounds the whole run (default `30m`).
- Every optional builder setting has a matching `PROXMOX_*` variable
  (`PROXMOX_CTID`, `PROXMOX_VLAN`, `PROXMOX_NETWORK_IP`, `PROXMOX_GATEWAY`,
  `PROXMOX_BACKUP_PIGZ`, ...) — see the doc comments in
  `builder/lxc/integration_test.go` for the full list.
- Both full-build tests clean up after themselves (destroying the
  container/template and, for the vzdump path, the backup file) once the
  test finishes, via `t.Cleanup`.

### Linting

```bash
make lint
```

### Full Development Cycle

```bash
make deps    # Download dependencies
make fmt     # Format code
make lint    # Run linter
make test    # Run tests
make build   # Build binary
```

## Releasing

Push a tag to trigger the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This will build binaries for Linux, macOS, and Windows (both amd64 and arm64) and create a GitHub release.
