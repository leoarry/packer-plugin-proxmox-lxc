<!--
  Include a short overview about the plugin.

  This document is a great location for creating a table of contents for each
  of the components the plugin may provide. This document should load automatically
  when navigating to the docs directory for a plugin.
-->

The Proxmox LXC Packer plugin enables you to build LXC container templates
on Proxmox VE using HashiCorp Packer. It connects to a Proxmox host via SSH,
creates a temporary LXC container from a template, runs provisioners inside it,
creates a backup of the configured container, and cleans up.

### Installation

To install this plugin, copy and paste this code into your Packer configuration, then run [`packer init`](https://www.packer.io/docs/commands/init).

```hcl
packer {
  required_plugins {
    proxmox-lxc = {
      source  = "github.com/leoarry/proxmox-lxc"
      version = ">= 0.1.0"
    }
  }
}
```

Alternatively, you can use `packer plugins install` to manage installation of this plugin.

```sh
$ packer plugins install github.com/leoarry/proxmox-lxc
```

**Note: Update to Packer Plugin Installation**

With the new Packer release starting from version 1.14.0, the `packer init` command will automatically install official plugins from the [HashiCorp release site.](https://releases.hashicorp.com/)

Going forward, to use newer versions of official Packer plugins, you'll need to upgrade to Packer version 1.14.0 or later. If you're using an older version, you can still install plugins, but as a workaround, you'll need to [manually install them using the CLI.](https://developer.hashicorp.com/packer/docs/plugins/install#manually-install-plugins-using-the-cli)

There is no change to the syntax or commands for installing plugins.

### Components

#### Builders

- [proxmox-lxc](/packer/integrations/leoarry/proxmox-lxc/latest/components/builder/proxmox-lxc) - The Proxmox LXC builder builds LXC container templates on a Proxmox VE host. It creates a temporary LXC container, runs provisioners inside it, then creates a `.tar.gz` backup that can be used as a new LXC template.
