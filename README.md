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
  memory       = 1024
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
| `memory` | int | `1024` | Memory in MB |
| `cores` | int | `2` | Number of CPU cores |
| `root_password` | string | `"changeme"` | Root password for the container |
| `unprivileged` | bool | `true` | Create unprivileged container |
| `features` | string | `"nesting=1"` | LXC features (e.g., `nesting=1,keyctl=1`) |
| `rootfs_size` | string | `"2"` | Root filesystem size in GB |
| `backup_name` | string | auto-generated | Name for the backup/template |
| `backup_dir` | string | `"/var/lib/vz/template/cache"` | Backup destination directory |
| `ctid` | string | auto-assigned | Specific CTID to use |
| `lxc_config` | string | - | Additional LXC config to merge |
| `ssh_timeout` | string | `"5m"` | SSH connection timeout |

## Examples

See the `example/` directory for complete examples:

- `example/basic.pkr.hcl` - Basic Ubuntu template build
- `example/advanced.pkr.hcl` - Advanced setup with SSH key auth and k3s installation

## How It Works

The plugin automates the following steps on your Proxmox host:

1. Connects to Proxmox via SSH
2. Gets the next available CTID (or uses configured one)
3. Creates an LXC container from the specified template
4. Merges custom LXC config (if provided)
5. Starts the container
6. Runs provisioners inside the container
7. Creates a backup of the container via `vzdump`
8. Stops and destroys the container (leaving the backup)

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

## Project Structure

```
packer-plugin-proxmox-lxc/
├── main.go                 # Plugin entry point
├── Makefile                # Build and test targets
├── .goreleaser.yml       # Release automation
├── builder/
│   └── lxc/
│       ├── builder.go             # Builder implementation
│       ├── config.go              # Configuration struct
│       ├── config.hcl2spec.go      # HCL2 spec (auto-generated)
│       ├── communicator.go         # SSH communicator implementations
│       ├── run_command.go          # Helper functions
│       ├── step_connect.go         # Connect to Proxmox host
│       ├── step_get_ctid.go       # Get next available CTID
│       ├── step_create_container.go  # Create LXC container
│       ├── step_merge_config.go    # Merge custom LXC config
│       ├── step_start_container.go  # Start container
│       ├── step_setup_container_comm.go  # Setup container communicator
│       ├── step_backup_container.go   # Backup container via vzdump
│       ├── step_destroy_container.go  # Destroy container
│       └── artifact.go            # Artifact implementation
├── version/
│   └── version.go        # Version info
├── tests/
│   └── lxc/             # Test files
├── example/               # Example configurations
└── .github/
    └── workflows/        # CI/CD workflows
```

## Releasing

Push a tag to trigger the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This will build binaries for Linux, macOS, and Windows (both amd64 and arm64) and create a GitHub release.
