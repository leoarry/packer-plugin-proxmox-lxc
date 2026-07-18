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

`make generate` regenerates `config.hcl2spec.go` from the `//go:generate packer-sdc mapstructure-to-hcl2 -type Config` directive in `config.go`, and rebuilds `.web-docs` from `docs/`. Run this after changing `Config` fields.

## Architecture

### Multistep build pipeline

`Builder.Run` (`builder/lxc/builder.go`) assembles a `multistep.Runner` (from `packer-plugin-sdk/multistep`) with a fixed step sequence, stored in a shared `multistep.BasicStateBag`:

```
stepConnect -> stepGetCTID -> stepCreateContainer -> stepMergeConfig ->
stepStartContainer -> stepSetupContainerComm -> commonsteps.StepProvision ->
  [backup_method == "template"]  stepCreateTemplate
  [backup_method == "vzdump"]    stepBackupContainer -> stepDestroyContainer
```

Each step is a separate file (`step_*.go`) implementing `Run(ctx, state) multistep.StepAction` and `Cleanup(state)`. Steps communicate purely through the state bag (`config`, `ui`, `communicator`, `ctid`, `container_reused`, `container_templated`, `backup_path`, etc.) — read `builder.go` to see which keys are set/consumed where before touching step ordering. `Builder.steps` is injectable (`setSteps`) so tests can substitute fake steps without exercising the real pipeline.

Cleanup on failure is defensive: `stepCreateContainer.Cleanup` best-effort stops/destroys the container unless it was reused, already destroyed, or successfully converted to a template — check the `state.GetOk(...)` guards there before adding new "keep the container" conditions elsewhere.

### Communicator layering (two levels of "run a command")

There are two different targets a command can run against, unified behind the `CommandRunner` interface (`RunCommand(ctx, command, stdout, stderr) error`):

1. **`sshCommunicator`** (`communicator.go`) — runs commands on the Proxmox *host* over SSH (via `SSHSessionProvider`, wrapping `golang.org/x/crypto/ssh`). Used for `pct`/`pvesh`/`vzdump` host-level commands.
2. **`pctExecCommunicator`** — wraps a parent `sshCommunicator` and prefixes commands with `pct exec <ctid> --`, running them *inside* the container. Created by `stepSetupContainerComm`, which swaps `state["communicator"]` from the host-level `sshCommunicator` to this container-level one and stashes the original under `state["proxmox_comm"]` (steps/cleanup after that point need the host-level runner for `pct`/`vzdump` commands, not the container one).

Both also implement the packer SDK's `Communicator` interface (`Start`/`Wait`/`Upload`/`Download`/...) so `commonsteps.StepProvision` can run provisioners inside the container.

### CTID auto-assignment race

`pvesh get /cluster/nextid` (`fetchNextCTID` in `step_get_ctid.go`) is a point-in-time query, not a reservation — concurrent builds against the same host can be handed the same "next" CTID. `stepCreateContainer` detects this (via `pct status` already succeeding, or `pct create` stderr matching `isCTIDConflict`) and re-rolls a fresh CTID with jittered backoff (`ctidRetryBackoff`), up to `maxCTIDConflictRetries` (5) — but only when the CTID was auto-assigned (`config.CTID == ""`); an explicitly configured CTID is reused, never retried. `concurrency_test.go` covers this behavior directly.

### Config

`Config` (`config.go`) is decoded via `packer-plugin-sdk`'s `mapstructure`/HCL2 machinery; `config.hcl2spec.go` is generated (do not hand-edit — run `make generate`). Validation and defaulting happens in `Config.Prepare`. Notable non-obvious fields:
- `backup_method`: `"vzdump"` (default — backs up then destroys the container) vs `"template"` (converts the container in place via `pct template`, container kept as the artifact).
- `rootfs_size` accepts human sizes (`"2"`, `"2GB"`, `2048MB"`) parsed by `parseSizeToGB`.
- `backup_pigz`: controls `vzdump` compression threading (`1` = auto/half-cores, `>1` = explicit thread count, `-1` = disable pigz, falls back to gzip with a warning if `pigz` isn't installed).
- `ctid`: if unset, auto-assigned with the retry-on-race behavior above.

### Artifact

`Artifact` (`artifact.go`) reports either a `vzdump` backup path or a `template` CTID depending on `backup_method`, constructed at the end of `Builder.Run` from state bag values left by the last step(s).

### Testing patterns

- Unit tests mock `CommandRunner`/`Communicator` (`mock_test.go`: `mockCommandRunner` replays a queued sequence of (stdout, stderr, error) tuples per call and records every command string — assert against `.calls`). No real SSH or Proxmox host is touched.
- `steps_test.go` and `builder_test.go` drive individual steps / the full `Builder.Run` against these mocks via `state.Put(...)`/`setSteps`.
- Integration tests (`integration_test.go`, tag `integration`) are the only tests that hit a real Proxmox host, gated by `PROXMOX_HOST` env var at runtime.
