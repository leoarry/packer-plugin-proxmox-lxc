package steps

// State bag keys shared by the steps and the builder. "config", "ui", "hook",
// "communicator" and "error" are also read by packer-plugin-sdk (e.g.
// commonsteps.StepProvision), so their values must not change.
const (
	StateConfig       = "config"       // *config.Config
	StateUI           = "ui"           // packersdk.Ui
	StateHook         = "hook"         // packersdk.Hook
	StateError        = "error"        // error that halts the build
	StateCommunicator = "communicator" // host runner, then the container one after SetupContainerComm
	StateProxmoxComm  = "proxmox_comm" // host runner, set by SetupContainerComm
	StateSSHClient    = "ssh_client"   // *ssh.Client, closed by Connect.Cleanup

	StateCTID               = "ctid"                // string
	StateContainerReused    = "container_reused"    // bool: existing container reused, never destroyed
	StateContainerDestroyed = "container_destroyed" // bool
	StateContainerTemplated = "container_templated" // bool: converted with pct template, kept as the artifact
	StateBackupPath         = "backup_path"         // string: vzdump file on the host
	StateBackupName         = "backup_name"         // string
)
