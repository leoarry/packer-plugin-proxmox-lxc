# Copyright (c) 2024
# SPDX-License-Identifier: MIT

# For full specification on the configuration of this file visit:
# https://github.com/hashicorp/integration-template#metadata-configuration
integration {
  name = "Proxmox LXC"
  description = "Build LXC container templates on Proxmox VE with HashiCorp Packer."
  identifier = "github.com/leoarry/proxmox-lxc"
  flags = ["hcp-ready"]
  docs {
    process_docs = true
    readme_location = "./README.md"
    external_url = "https://github.com/leoarry/packer-plugin-proxmox-lxc"
  }
  license {
    type = "MIT"
    url = "https://github.com/leoarry/packer-plugin-proxmox-lxc/blob/main/LICENSE"
  }
  component {
    type = "builder"
    name = "Proxmox LXC"
    slug = "proxmox-lxc"
  }
}
