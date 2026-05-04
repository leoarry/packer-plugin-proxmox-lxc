package version

import "github.com/hashicorp/packer-plugin-sdk/version"

var (
	Version           = "0.1.0"
	VersionPrerelease = ""
	VersionMetadata   = ""

	// PluginVersion initializes the plugin version info.
	PluginVersion = version.NewPluginVersion(Version, VersionPrerelease, VersionMetadata)
)
