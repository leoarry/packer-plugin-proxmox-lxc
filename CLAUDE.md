# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A HashiCorp Packer plugin (`packer-plugin-proxmox-lxc`) that builds LXC container templates on Proxmox VE. It connects to a Proxmox host over **plain SSH** (not the Proxmox API) and drives `pct`/`pvesh`/`vzdump` shell commands directly. There is no Proxmox API client in this codebase — everything is SSH + CLI commands with parsed stdout/stderr.

## Commands

```bash
make build              # go build -> ./packer-plugin-proxmox-lxc
make test               # go test -v -race -coverprofile=coverage.txt ./...  (unit tests, mocked, no infra needed)
make test-short         # go test -v -short ./...
make lint               # golangci-lint run ./...
make fmt / check-fmt    # gofmt -w . / verify formatting
make deps               # go mod download && go mod tidy
```

Run a single test:
```bash
go test ./builder/lxc/ -run TestName -v
```

### Integration tests (build tag `integration`)

`builder/lxc/integration_test.go` exercises the plugin against a **real** Proxmox host. Two checks are safe to run without infra (and run in CI):

```bash
make vet-integration     # go vet -tags integration ./...  (compile-only)
make test-integration    # go test -tags integration -v ./... -run TestIntegration  (self-skips without PROXMOX_HOST)
```

To run for real against a cluster, pass `PROXMOX_*` vars to `test-integration-full` (see the Makefile and doc comments in `integration_test.go` for the full variable list, e.g. `PROXMOX_HOST`, `PROXMOX_USER`, `PROXMOX_PASSWORD`/`PROXMOX_KEY_PATH`, `PROXMOX_TEMPLATE`). `RUN` narrows which `TestIntegration*` tests run; `TIMEOUT` bounds the run (default `30m`). Both full-build tests clean up their container/backup via `t.Cleanup`.

- `ssh_user` (and `PROXMOX_USER`) must be a real Linux login (e.g. `root`), not Proxmox's `user@realm` API format — this plugin connects over plain SSH, not the Proxmox API.

### Codegen

`make generate` regenerates `builder/lxc/internal/config/config.hcl2spec.go` from the `//go:generate packer-sdc mapstructure-to-hcl2 -type Config` directive in `config.go` next to it, and rebuilds `.web-docs` from `docs/`. Run this after changing `Config` fields.

## Architecture

### Package layout

```
builder/lxc/                 Builder (builder.go), Artifact, integration tests
builder/lxc/internal/config  Config: decoding, defaults, validation (+ generated hcl2spec)
builder/lxc/internal/steps   the multistep steps and the state-bag key constants (state.go)
builder/lxc/internal/remote  SSH / pct exec communicators
builder/lxc/internal/testutil test doubles shared across packages (imported by tests only)
```

Imports only go downward: `lxc` -> `steps` -> `config`, `remote`. `config` and `remote` import nothing from this repo, so `steps` can't import `lxc` (import cycle) — anything a step needs must live in `steps`, `config` or `remote`.

### Multistep build pipeline

`Builder.Run` (`builder/lxc/builder.go`) assembles a `multistep.Runner` (from `packer-plugin-sdk/multistep`) with a fixed step sequence, stored in a shared `multistep.BasicStateBag`:

```
steps.Connect -> steps.GetCTID -> steps.CreateContainer -> steps.MergeConfig ->
steps.StartContainer -> steps.SetupContainerComm -> commonsteps.StepProvision ->
  [backup_method == "template"]  steps.CreateTemplate
  [backup_method == "vzdump"]    steps.BackupContainer -> steps.DestroyContainer
```

Each step is a separate file in `internal/steps` (`connect.go`, `create_container.go`, ...) implementing `Run(ctx, state) multistep.StepAction` and `Cleanup(state)`. Steps communicate purely through the state bag, using the key constants in `internal/steps/state.go` (`steps.StateConfig`, `StateCommunicator`, `StateCTID`, `StateContainerReused`, ...) rather than string literals — grep for a constant to see where a key is set/consumed before touching step ordering. `config`, `ui`, `hook`, `communicator` and `error` are also read by the SDK, so never change their values. `Builder.steps` is injectable (`setSteps`) so tests can substitute fake steps without exercising the real pipeline.

Cleanup on failure is defensive: `steps.CreateContainer.Cleanup` best-effort stops/destroys the container unless it was reused, already destroyed, or successfully converted to a template — check the `state.GetOk(...)` guards there before adding new "keep the container" conditions elsewhere.

### Communicator layering (two levels of "run a command")

There are two different targets a command can run against, unified behind the `remote.CommandRunner` interface (`RunCommand(ctx, command, stdout, stderr) error`). Both communicators live in their own package, `builder/lxc/internal/remote`, which knows nothing about `Config` or the state bag; steps build them with `remote.NewSSHCommunicator` / `remote.NewPctExecCommunicator`:

1. **`remote.SSHCommunicator`** (`internal/remote/communicator.go`, sessions in `internal/remote/ssh_session.go`) — runs commands on the Proxmox *host* over SSH (via `SSHSessionProvider`, wrapping `golang.org/x/crypto/ssh`). Used for `pct`/`pvesh`/`vzdump` host-level commands.
2. **`remote.PctExecCommunicator`** — wraps a parent `SSHCommunicator` and prefixes commands with `pct exec <ctid> --`, running them *inside* the container. Created by `steps.SetupContainerComm`, which swaps `state["communicator"]` from the host-level `SSHCommunicator` to this container-level one and stashes the original under `state["proxmox_comm"]` (steps/cleanup after that point need the host-level runner for `pct`/`vzdump` commands, not the container one).

Both also implement the packer SDK's `Communicator` interface (`Start`/`Wait`/`Upload`/`Download`/...) so `commonsteps.StepProvision` can run provisioners inside the container.

### CTID auto-assignment race

`pvesh get /cluster/nextid` (`fetchNextCTID` in `internal/steps/get_ctid.go`) is a point-in-time query, not a reservation — concurrent builds against the same host can be handed the same "next" CTID. `steps.CreateContainer` detects this (via `pct status` already succeeding, or `pct create` stderr matching `isCTIDConflict`) and re-rolls a fresh CTID with jittered backoff (`ctidRetryBackoff`), up to `maxCTIDConflictRetries` (5) — but only when the CTID was auto-assigned (`cfg.CTID == ""`); an explicitly configured CTID is reused, never retried. `internal/steps/concurrency_test.go` covers this behavior directly.

### Config

`Config` (`internal/config/config.go`) is decoded via `packer-plugin-sdk`'s `mapstructure`/HCL2 machinery; `config.hcl2spec.go` is generated (do not hand-edit — run `make generate`). Validation and defaulting happens in `Config.Prepare`. Notable non-obvious fields:
- `backup_method`: `"vzdump"` (default — backs up then destroys the container) vs `"template"` (converts the container in place via `pct template`, container kept as the artifact).
- `rootfs_size` accepts human sizes (`"2"`, `"2GB"`, `2048MB"`) parsed by `parseSizeToGB`.
- `backup_pigz`: controls `vzdump` compression threading (`1` = auto/half-cores, `>1` = explicit thread count, `-1` = disable pigz, falls back to gzip with a warning if `pigz` isn't installed).
- `ctid`: if unset, auto-assigned with the retry-on-race behavior above.

### Artifact

`Artifact` (`artifact.go`) reports either a `vzdump` backup path or a `template` CTID depending on `backup_method`, constructed at the end of `Builder.Run` from state bag values left by the last step(s).

### Testing patterns

- Unit tests mock `remote.CommandRunner` with `testutil.MockRunner` (`internal/testutil`): it replays a queued sequence of (stdout, stderr, error) tuples per call (`Outputs`/`Stderrs`/`Errors`) and records every command string — assert against `.Calls`. No real SSH or Proxmox host is touched. Put test doubles needed by more than one package in `internal/testutil` (`MockRunner`, `UI`, `BoolPtr`) — it is a normal package imported only by tests, so it never reaches the plugin binary. Package-specific doubles stay in that package's `mock_test.go` (SSH session mocks in `internal/remote`, `someError` and communicator mocks in `internal/steps`).
- `internal/steps/*_test.go` (one per step) and `builder_test.go` drive individual steps / the full `Builder.Run` against these mocks via `state.Put(...)`/`setSteps`.
- Integration tests (`integration_test.go`, tag `integration`) are the only tests that hit a real Proxmox host, gated by `PROXMOX_HOST` env var at runtime.
