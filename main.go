package main

import (
	"github.com/hashicorp/packer-plugin-sdk/plugin"

	"github.com/leoarry/packer-plugin-proxmox-lxc/builder/lxc"
	"github.com/leoarry/packer-plugin-proxmox-lxc/version"
)

func main() {
	set := plugin.NewSet()
	set.SetVersion(version.PluginVersion)
	set.RegisterBuilder(plugin.DEFAULT_NAME, &lxc.Builder{})
	if err := set.Run(); err != nil {
		panic(err)
	}
}
