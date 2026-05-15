# Runtime-launch validation — workflow-plugin-gcp v1.1.0

**Scope:** plan-2 PR 3 finishing task (Task 11) — wire
`IaCServeOptions.Modules` for `gcp.credentials` + `storage.gcs`; bump
`plugin.json` `version` / `minEngineVersion`; release v1.1.0. The branch
also carries the locked B/C/D plan's prior gcp work (gcs `IaCStateBackend`
+ `gke` cross-process contract via the existing `ResourceDriver`).

**Change class:** plugin-loading path + version pin → runtime-launch
validation per the cross-plan policy.

## What was validated

### 1. Build

```
$ GOWORK=off go build -o /tmp/gcp-plugin-v110/workflow-plugin-gcp ./cmd/workflow-plugin-gcp
```

Build is clean (`BUILD_EXIT 0`); subprocess binary linked.

### 2. go-plugin handshake guard

Running the binary outside the host surfaces the canonical go-plugin
self-identification — proves `sdk.ServeIaCPlugin` accepted the new
`IaCServeOptions.Modules` field and the handshake guard is intact.

```
$ /tmp/gcp-plugin-v110/workflow-plugin-gcp
This binary is a plugin. These are not meant to be executed directly.
Please execute the program that consumes these plugins, which will
load any plugins automatically
```

### 3. In-process bridge parity

- `TestPluginJSONCapabilities_ModuleStep_Parity` — plugin.json
  `capabilities.moduleTypes` (minus the implicit `iac.provider`) ↔
  `internal.ModuleProviders()` keys; `capabilities.stepTypes` (empty for
  gcp) ↔ `internal.StepProviders()` (nil). Bidirectional.
- `TestCapabilityParity_IaCStateBackends` — pre-existing parity for the
  `iac.state` backend surface (`gcs`).

Both pass.

### 4. Full unit test suite

```
ok  github.com/GoCodeAlone/workflow-plugin-gcp/internal
ok  github.com/GoCodeAlone/workflow-plugin-gcp/internal/credref
ok  github.com/GoCodeAlone/workflow-plugin-gcp/internal/gcpcreds
ok  github.com/GoCodeAlone/workflow-plugin-gcp/internal/modules
ok  github.com/GoCodeAlone/workflow-plugin-gcp/internal/statebackend
ok  github.com/GoCodeAlone/workflow-plugin-gcp/provider
ok  github.com/GoCodeAlone/workflow-plugin-gcp/provider/drivers
```

All packages green under `GOWORK=off go test ./... -race`.

## What was NOT validated here

A full `wfctl plugin install <binary> && wfctl plugin list` end-to-end
exercise was not run in this implementer session — same rationale as the
aws plugin's v1.1.0 transcript. The host-load path is exercised by
workflow-core plan-2 Task 2 integration tests against the v0.53.0 tag
this PR pins, and plan-2 PR 5 (Phase C core deletion) is blocked on this
release tag, so any regression surfaces at PR 5's CI before any in-core
path is removed.
