<!--
  Include a short description about the builder. This is a good place
  to call out what the builder does, and any requirements for the given
  builder environment.
-->

The `proxmox-lxc` Packer builder builds LXC container templates on a Proxmox VE host.
It connects to the Proxmox host via SSH, creates a temporary LXC container from
a specified template, runs provisioners inside the container, creates a
`.tar.gz` backup using `vzdump`, and then destroys the temporary container
leaving behind a ready-to-deploy LXC template.

The builder communicates with Proxmox over SSH and does not require
the Proxmox API. Any machine with SSH access to the Proxmox host can
run the build.

## Basic Example

Below is a fully functioning example that creates an Ubuntu LXC template.

**HCL2**

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
  # SSH connection to Proxmox host
  ssh_host = "192.168.1.100"
  ssh_user = "root"
  ssh_password = "your-password"

  # LXC template to use as base
  template = "local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst"
  storage  = "local-lvm"
  memory   = 2048
  cores    = 2

  # Backup settings
  backup_name = "ubuntu-22.04-template"
  backup_dir = "/var/lib/vz/template/cache"
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

## Configuration Reference

Configuration options are organized below into two categories: required and
optional. Within each category, the available options are alphabetized and
described.

### Required

- `ssh_host` (string) - Proxmox host IP address or hostname.

- `ssh_user` (string) - SSH user for the Proxmox host (e.g., `root`).

- `template` (string) - LXC template to use as base.
  Example: `local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst`

- `ssh_password` (string) - SSH password for authentication.
  Required if `ssh_key_path` is not set.

- `ssh_key_path` (string) - Path to SSH private key for authentication.
  Required if `ssh_password` is not set.

### Optional

- `backup_dir` (string) - Directory where the backup template will be stored
  on the Proxmox host.
  Default: `/var/lib/vz/template/cache`

- `backup_name` (string) - Name for the resulting backup/template file.
  If not set, a name is auto-generated from the CTID and timestamp.
  Supports HCL2 template functions like `timestamp()` and `formatdate()` for
  dynamic naming, e.g., `"my-template_${formatdate("2006-01-02_15-04", timestamp())}_debian_12.7-1_amd64"`

- `bridge` (string) - Network bridge for the container.
  Default: `"vmbr0"`

- `cores` (int) - Number of CPU cores for the container.
  Default: `2`

- `ctid` (string) - Specific CTID to use for the container.
  If not set, the next available CTID is automatically assigned.

- `features` (string) - LXC features to enable (e.g., `nesting=1,keyctl=1`).
  Default: `"nesting=1"`

- `firewall` (bool) - Enable the Proxmox firewall on the container's
  network interface.
  Default: `false`

- `gateway` (string) - Gateway IP address for the container's network
  interface. Only used when `network_ip` is set to a static IP.

- `lxc_config` (string) - Additional LXC configuration to merge into the
  container configuration.

- `memory` (int) - Memory in MB for the container.
  Default: `2048`

- `network_ip` (string) - Network IP configuration for the container's
  `net0` interface. One of `"dhcp"`, `"manual"`, or a static IP in CIDR
  notation, e.g. `"192.168.1.50/24"`.
  Default: `"dhcp"`

- `network_mtu` (int) - MTU for the container's network interface.
  If not set, the bridge/interface default is used.

- `root_password` (string) - Root password for the container.
  Default: `"changeme"`

- `rootfs_size` (string) - Root filesystem size. Accepts plain numbers
  (GB) or values with units like `2GB`, `2048MB`, `1TB`.
  Default: `"8"` (8 GB)

- `ssh_port` (int) - SSH port for connecting to the Proxmox host.
  Default: `22`

- `ssh_timeout` (string) - Timeout for SSH connections during provisioning.
  Example: `"5m"`.
  Default: `"5m"`

- `storage` (string) - Storage backend for the container.
  Default: `"local-lvm"`

- `unprivileged` (bool) - Create an unprivileged container.
  Default: `true`

- `vlan` (int) - VLAN tag for the container's network bridge, between `1`
  and `4094`. If not set, the interface is not VLAN-tagged.

## How It Works

The plugin automates the following steps on your Proxmox host:

1. Connects to Proxmox via SSH
2. Gets the next available CTID (or uses the configured one)
3. Creates an LXC container from the specified template
4. Merges custom LXC config (if provided)
5. Starts the container
6. Runs provisioners inside the container
7. Creates a backup of the container via `vzdump`
8. Stops and destroys the container (leaving the backup as a template)

## Examples

See the `example/` directory in the repository for complete examples:

- `example/basic.pkr.hcl` - Basic Ubuntu template build
- `example/advanced.pkr.hcl` - Advanced setup with SSH key auth and k3s installation
