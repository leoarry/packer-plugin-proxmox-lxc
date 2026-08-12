<!--
  Include a short description about the builder. This is a good place
  to call out what the builder does, and any requirements for the given
  builder environment.
-->

The `proxmox-lxc` Packer builder builds LXC container templates on a Proxmox VE host.
It connects to the Proxmox host via SSH, creates a temporary LXC container from
a specified template, runs provisioners inside the container, and then
finalizes the result using one of two methods (`backup_method`):

- `vzdump` (default) - creates a `.tar.gz` backup of the container via
  `vzdump`, then stops and destroys the temporary container, leaving the
  backup file behind as the template.
- `template` - stops the container and converts it directly into a Proxmox
  CT template via `pct template`. The container itself is left in place on
  the Proxmox host (not destroyed) and becomes the artifact; it can be
  cloned with `pct clone`.

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
  on the Proxmox host. Only used when `backup_method` is `"vzdump"`.
  Default: `/var/lib/vz/template/cache`

- `backup_method` (string) - How to finalize the built container: `"vzdump"`
  creates a `.tar.gz` backup and destroys the container; `"template"`
  converts the container itself into a Proxmox CT template via
  `pct template` and leaves it in place.
  Default: `"vzdump"`

- `backup_name` (string) - Name for the resulting backup file. Only used
  when `backup_method` is `"vzdump"`. If not set, a name is auto-generated
  from the CTID and timestamp.
  Supports HCL2 template functions like `timestamp()` and `formatdate()` for
  dynamic naming, e.g., `"my-template_${formatdate("2006-01-02_15-04", timestamp())}_debian_12.7-1_amd64"`

- `backup_compression` (string) - Compression algorithm passed to `vzdump`,
  one of `"gzip"`, `"lzo"` or `"zstd"`. `zstd` decompresses faster and
  produces a smaller artifact, and is Proxmox VE's own `vzdump` default
  since 6.2. The choice determines the artifact's extension
  (`.tar.gz`, `.tar.lzo`, `.tar.zst`). `backup_pigz` applies to `gzip`
  only and is ignored for the others. Only used when `backup_method`
  is `"vzdump"`.
  Default: `"gzip"`

- `backup_pigz` (int) - Number of pigz threads to use for vzdump backup
  compression instead of plain gzip, for faster backups on multi-core
  hosts. `1` (default) auto-selects half of the host's cores; `>1` uses
  that many threads; `-1` explicitly disables pigz and always uses plain
  gzip. For any value `> 0`, the Proxmox host is checked for a `pigz`
  binary first; if it isn't installed, the build falls back to plain
  gzip with a warning instead of failing. Only used when `backup_method`
  is `"vzdump"`.
  Default: `1` (auto, half of cores)

- `bridge` (string) - Network bridge for the container.
  Default: `"vmbr0"`

- `cores` (int) - Number of CPU cores for the container.
  Default: `2`

- `ctid` (string) - Specific CTID to use for the container. If it already
  exists, the build reuses it rather than creating a new one.
  If not set, the next available CTID is automatically assigned; if that
  CTID turns out to be claimed by a concurrent build running against the
  same Proxmox host, a fresh CTID is fetched and retried automatically
  (up to 5 times) rather than failing the build or reusing the other
  build's container.

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
7. Finalizes the container using the configured `backup_method`:
   - `vzdump` (default): creates a backup of the container via `vzdump`,
     then stops and destroys the container (leaving the backup as a template)
   - `template`: stops the container and converts it into a Proxmox CT
     template via `pct template` (the container itself is left in place)

## Examples

See the `example/` directory in the repository for complete examples:

- `example/basic.pkr.hcl` - Basic Ubuntu template build
- `example/advanced.pkr.hcl` - Advanced setup with SSH key auth and k3s installation
- `example/ct-template.pkr.hcl` - Produces a Proxmox CT template (`backup_method = "template"`) instead of a vzdump backup
