# Basic Proxmox LXC template build
packer {
  required_plugins {
    proxmox-lxc = {
      version = ">=0.1.0"
      source  = "github.com/leoarry/proxmox-lxc"
    }
  }
}

source "proxmox-lxc" "basic" {
  # SSH connection
  ssh_host        = "192.168.1.100"
  ssh_user        = "root"
  ssh_password = "changeme"

  # LXC template
  template     = "local:vztmpl/debian-12-standard_12.0-1_amd64.tar.zst"
}

build {
  name = "proxmox-lxc-basic"
  sources = ["source.proxmox-lxc.basic"]

  provisioner "shell" {
    inline = [
      "apt-get update",
      "apt-get upgrade -y",
      "apt-get install -y curl wget vim git"
    ]
  }
}
