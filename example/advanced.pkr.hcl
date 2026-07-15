# Advanced Proxmox LXC template build with SSH key authentication
packer {
  required_plugins {
    proxmox-lxc = {
      version = ">= 0.1.0"
      source  = "github.com/leoarry/proxmox-lxc"
    }
  }
}

source "proxmox-lxc" "debian" {
  # SSH connection via SSH key
  ssh_host        = "192.168.1.100"
  ssh_user        = "root"
  ssh_key_path = "~/.ssh/id_rsa"

  # LXC template
  template      = "local:vztmpl/debian-12-standard_12.0-1_amd64.tar.zst"
  storage       = "local-lvm"
  bridge        = "vmbr0"

  # Network settings: tag the interface with VLAN 100 and assign a
  # static IP instead of DHCP.
  vlan        = 100
  network_ip  = "192.168.100.50/24"
  gateway     = "192.168.100.1"
  firewall    = true

  memory        = 4096
  cores         = 2
  root_password = "changeme"
  unprivileged  = true
  features      = "nesting=1,keyctl=1"
  rootfs_size   = "16"

  # Custom LXC config
  lxc_config = <<-EOF
    lxc.cgroup2.devices.allow: c 10:200 rwm
    lxc.mount.entry: /dev/net dev/net none bind,create=dir
  EOF

  # Backup settings (supports HCL2 templating for dynamic names)
  backup_name = "debian-node_${formatdate("2006-01-02_15-04", timestamp())}"
  backup_dir  = "/var/lib/vz/template/cache"
  ctid        = "100"  # Use specific CTID

  # SSH timeout for provisioning
  ssh_timeout = "10m"
}

build {
  name = "proxmox-lxc-debian"
  sources = ["source.proxmox-lxc.debian"]

  # Install additional packages
  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get install -y curl wget vim git",
      "apt-get clean",
    ]
  }
}
