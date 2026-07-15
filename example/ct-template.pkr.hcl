# Proxmox LXC build that produces a Proxmox CT template instead of a
# vzdump backup. The provisioned container is stopped and converted in
# place via `pct template`, and is left on the Proxmox host as a
# clonable template rather than being destroyed.
packer {
  required_plugins {
    proxmox-lxc = {
      version = ">= 0.1.0"
      source  = "github.com/leoarry/proxmox-lxc"
    }
  }
}

source "proxmox-lxc" "ubuntu-ct-template" {
  # SSH connection
  ssh_host     = "192.168.1.100"
  ssh_user     = "root"
  ssh_password = "changeme"

  # LXC template to use as base
  template = "local:vztmpl/ubuntu-22.04-standard_22.04-1_amd64.tar.zst"
  storage  = "local-lvm"
  memory   = 2048
  cores    = 2

  # Produce a Proxmox CT template instead of a vzdump backup.
  # backup_name/backup_dir are unused with this method.
  backup_method = "template"
  ctid          = "9000" # Fixed CTID makes the resulting template easy to find/clone
}

build {
  sources = ["source.proxmox-lxc.ubuntu-ct-template"]

  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get upgrade -y",
      "apt-get install -y curl wget vim",
    ]
  }
}
