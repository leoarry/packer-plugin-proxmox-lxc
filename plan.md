# Packer Plugin Compliance Plan - Implementation Summary

## Context
Make `packer-plugin-proxmox-lxc` fully compliant with HashiCorp Packer plugin standards for official registration as a Packer integration.

## What Was Done

### 1. ✅ Fixed `.goreleaser.yml` — Binary & Checksum Naming
**File**: `.goreleaser.yml`

Updated to produce correctly named artifacts:
- Binary: `packer-plugin-proxmox-lxc_{{ .Version }}_x5.0_{{ .Os }}_{{ .Arch }}`
- Archives: `{{ .ProjectName }}_{{ .Version }}_x5.0_{{ .Os }}_{{ .Arch }}.zip`
- Checksums: `{{ .ProjectName }}_{{ .Version }}_x5.0_SHA256SUMS`

Verified with `goreleaser release --snapshot --clean`:
```
dist/packer-plugin-proxmox-lxc_v0.0.0-next_x5.0_darwin_amd64.zip
dist/packer-plugin-proxmox-lxc_v0.0.0-next_x5.0_linux_amd64.zip
dist/packer-plugin-proxmox-lxc_v0.0.0-next_x5.0_windows_amd64.zip
dist/packer-plugin-proxmox-lxc_v0.0.0-next_x5.0_SHA256SUMS
```

### 2. ✅ Created Proper `.web-docs/` Directory Structure
Based on `hashicorp/packer-plugin-scaffolding` and `hashicorp/packer-plugin-docker` templates.

**Files created**:
- `.web-docs/metadata.hcl` — Integration metadata for Packer portal registration
- `.web-docs/README.md` — Plugin documentation with installation instructions and component links
- `.web-docs/scripts/compile-to-webdocs.sh` — Script for compiling MDX docs to Markdown
- `.web-docs/components/builder/proxmox-lxc/README.md` — Full builder documentation with config reference

### 3. ✅ Fixed `go.mod` — SDK Upgrade & Replace Directives
**File**: `go.mod`

- Upgraded `packer-plugin-sdk` from `v0.5.4` to `v0.6.7`
- Kept `replace` directives for `go-cty` and `hcl/v2` (required due to GobEncode compatibility issue)
- The `replace` directives are necessary for the build and don't affect `packer plugins install`

### 4. ✅ Updated GitHub Workflows to Go 1.24
**Files**:
- `.github/workflows/build.yml` — Updated to `go-version: 1.24`
- `.github/workflows/release.yml` — Updated to `go-version: 1.24`

### 5. ✅ Updated Makefile
**File**: `Makefile`

- Updated `PACKER_PLUGIN_DIR` to `$(HOME)/.config/packer/plugins` (modern path)
- `install-plugin` target correctly uses `packer plugins install --path`

### 6. ✅ Fixed Plugin Builder Registration
**File**: `main.go`

Reverted to `plugin.DEFAULT_NAME` (as user requested — using `"proxmox-lxc"` directly caused issues with `packer build`).

**Verified describe output**:
```json
{"version":"0.1.0","sdk_version":"0.6.7","api_version":"x5.0","builders":["proxmox-lxc"],"post_processors":[],"provisioners":[],"datasources":[],"protocol_version":"v2"}
```

### 7. ✅ Verified Build & Plugin Compliance
- `go build ./...` succeeds
- `./packer-plugin-proxmox-lxc describe` outputs valid JSON with correct fields
- GoReleaser snapshot produces correctly named artifacts

## Remaining Steps for Full Registration

1. **Push a semantic version tag**:
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

2. **Verify GitHub Release** creates correctly named binaries

3. **Test `packer init`** with:
   ```hcl
   packer {
     required_plugins {
       proxmox-lxc = {
         version = ">= 0.1.0"
         source  = "github.com/leoarry/proxmox-lxc"
       }
     }
   }
   ```

4. **Open integration request** with the Packer team to get listed on the integration portal

## Critical Files Modified
| File | Change |
|------|-------|
| `.goreleaser.yml` | Fixed binary/checksum naming for `x5.0` API |
| `.web-docs/metadata.hcl` | Created for integration registration |
| `.web-docs/README.md` | Created with proper docs (not referring to README) |
| `.web-docs/scripts/compile-to-webdocs.sh` | Created from docker/scaffolding template |
| `.web-docs/components/builder/proxmox-lxc/README.md` | Created full builder documentation |
| `go.mod` | Upgraded SDK to v0.6.7, kept replace directives |
| `.github/workflows/build.yml` | Updated to Go 1.24 |
| `.github/workflows/release.yml` | Updated to Go 1.24 |
| `Makefile` | Updated plugin dir to modern path |
| `main.go` | Reverted to `plugin.DEFAULT_NAME` |
