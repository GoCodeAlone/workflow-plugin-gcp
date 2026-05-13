# Design: GCP Plugin Typed-IaC Conformance Migration to v1.0.0

**Date:** 2026-05-13
**Author:** Claude Code (autonomous pipeline)
**Status:** Draft (cycle-2 revision — addresses cycle-1 findings)

## Context

workflow-plugin-gcp v0.1.4 has a flat `main.go` entrypoint that is not a real plugin
server — it prints `info` to stdout and exits. It does not call `sdk.Serve` or
`sdk.ServeIaCPlugin`. The workflow engine's strict-contracts force-cutover (v0.51.0+)
requires plugins to be served via `sdk.ServeIaCPlugin`, which auto-registers every typed
pb.IaCProvider*Server interface the server satisfies.

The existing provider layer (`provider/provider.go`, 13 drivers) is correct and complete.
The migration is purely infrastructure: create the typed gRPC server wrapper and a proper
plugin entrypoint.

## Precedent

- workflow-plugin-aws v1.0.0 (PR #11, merged 2026-05-13): single-PR force-cutover.
  Added `internal/iacserver.go` + `internal/resourcedriver_server.go`, restructured
  entrypoint to `cmd/workflow-plugin-aws/main.go` calling `sdk.ServeIaCPlugin`.
  Pinned workflow v0.51.7. Mirror exactly.

- workflow-plugin-digitalocean v1.0.1: same pattern, v0.51.2.

## Approach (single chosen option)

**Single-PR force-cutover mirroring AWS v1.0.0 / DO v1.0.1 exactly.**

The current flat `main.go` is replaced by a proper plugin entrypoint at
`cmd/workflow-plugin-gcp/main.go`. The `internal/` package is created with the typed
gRPC server. No compat shim. No string-dispatch surface.

Alternatives considered:
- **Keep flat main.go + add ServeIaCPlugin call** — rejected: the `main` package is in the
  repo root with `integration_test.go` as `package main_test`. Moving the entrypoint to
  `cmd/` is cleaner, matches the AWS pattern, and avoids a messy `package main` with tests
  in the same directory that test non-plugin behavior.
- **Two-PR approach** — rejected per `feedback_force_strict_contracts_no_compat`.

## Scope

### Phase 1 — Typed server layer (new package `internal/`)

**New files:**

| File | Action |
|------|--------|
| `internal/iacserver.go` | NEW: `gcpIaCServer` struct + all required + optional pb service methods |
| `internal/resourcedriver_server.go` | NEW: ResourceDriver CRUD dispatch per-type |
| `internal/iacserver_test.go` | NEW: unit tests for server methods (mock provider) |
| `internal/host_conformance_test.go` | NEW: typed-IaC load path smoke test |
| `internal/contracts/gcp.proto` | NEW: GCPProviderConfig proto message |
| `internal/contracts/gcp.pb.go` | NEW: generated pb (hand-rolled, mirrors aws.pb.go) |

**`gcpIaCServer` struct embeds** (mirrors AWS exactly):
```
pb.UnimplementedIaCProviderRequiredServer
pb.UnimplementedIaCProviderEnumeratorServer
pb.UnimplementedIaCProviderDriftDetectorServer
pb.UnimplementedIaCProviderCredentialRevokerServer
pb.UnimplementedIaCProviderMigrationRepairerServer
pb.UnimplementedIaCProviderValidatorServer
pb.UnimplementedIaCProviderDriftConfigDetectorServer
pb.UnimplementedResourceDriverServer
```

**What gets implemented** (methods `*GCPProvider` actually supports):
- All `IaCProviderRequiredServer` methods: `Initialize`, `Name`, `Version`,
  `Capabilities`, `Plan`, `Apply`, `Destroy`, `Status`, `Import`, `ResolveSizing`,
  `BootstrapStateBackend`
- `IaCProviderDriftDetectorServer.DetectDrift` AND `DetectDriftWithSpecs` —
  thin delegator pattern (both required for service auto-registration)
- `ResourceDriverServer` 9 CRUD methods

**What is left as Unimplemented** (forward-compat embed only):
- `EnumerateAll`, `EnumerateByTag` — no GCP tag-query implementation
- `RevokeProviderCredential` — no credential rotation
- `RepairDirtyMigration` — no migration repair
- `ValidatePlan` — no cross-resource plan validator
- `DetectDriftConfig` — separate service, Unimplemented

Marshalling pattern: JSON bytes for config/outputs — NO structpb.Struct, NO typed slices
in Outputs (all outputs are `map[string]any` marshalled to JSON bytes). Copied from AWS.

### Phase 2 — Entrypoint restructure

| File | Action |
|------|--------|
| `cmd/workflow-plugin-gcp/main.go` | NEW: `sdk.ServeIaCPlugin(internal.NewIaCServer(), ...)` |
| `main.go` | DELETE: old info-printer entrypoint (no plugin serves here) |
| `integration_test.go` | MOVE to `provider/integration_test.go` (package provider_test) |

**Atomic note:** `integration_test.go` is `package main_test` at root. Deleting `main.go`
(which defines `package main`) while leaving `integration_test.go` will fail to compile
(`package main_test` has no `package main` to attach to). Both must move atomically.

The integration tests use `wftest` mock steps — they do not test any behavior from
`main.go` itself and are purely provider-level tests. Moving to `provider/` makes sense.

### Phase 3 — Version/metadata updates

| File | Action |
|------|--------|
| `go.mod` | Bump `workflow v0.19.2` → `v0.51.7`; run `GOWORK=off go mod tidy`; **then immediately run `GOWORK=off go build ./...` as a compile blocker before writing any new code** — confirms wftest API compiles at v0.51.7 |
| `plugin.json` | Bump `version` to `1.0.0`, `minEngineVersion` to `0.51.0`; add `"moduleTypes": ["iac.provider"]` |
| `plugin.contracts.json` | NEW: `{"version":"v1","contracts":[{"kind":"module","type":"iac.provider","mode":"strict","config":"workflow.plugins.gcp.v1.GCPProviderConfig"}]}` |
| `provider/provider.go` | Add `NewGCPProviderConcrete() *GCPProvider` constructor (plan-phase finding from AWS retro) |
| `.goreleaser.yaml` | Update `main: .` → `main: ./cmd/workflow-plugin-gcp`; update `archives` section to include `plugin.json`, `plugin.contracts.json`, `LICENSE`; use only `provider.ProviderVersion` ldflag (do NOT add `internal.Version` — that var does not exist) |
| `scripts/update-plugin-version.sh` | Update sed pattern for `workflow-plugin-gcp` → unchanged (already correct) |

**Critical — goreleaser archives section:** The `archives` block MUST include the `files` stanza:
```yaml
archives:
  - id: default
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - plugin.json
      - plugin.contracts.json
      - LICENSE
```
Without this, the engine cannot load `plugin.contracts.json` from the release tarball and strict-contracts breaks at install time. Copying the AWS v1.0.0 pattern exactly.

**Critical — goreleaser ldflags:** Only one ldflag is needed:
```
-s -w -X github.com/GoCodeAlone/workflow-plugin-gcp/provider.ProviderVersion={{.Version}}
```
Do NOT copy the AWS spurious `-X github.com/GoCodeAlone/workflow-plugin-gcp/internal.Version={{.Version}}` — that var does not exist in GCP (or AWS) and the AWS version is a bug that silently succeeds only because Go ldflag linker ignores undefined symbols in some builds.

### Phase 4 — CI updates

| File | Action |
|------|--------|
| `.github/workflows/ci.yml` | Update `wfctl@v0.20.1` → `wfctl@v0.51.7` for strict-contracts gate; add `wfctl-strict-contracts` job matching AWS pattern |
| `.github/workflows/iac-host-conformance.yml` | NEW: copy from AWS v1.0.0, adapt test name to `TestWorkflowHostConformance_LoadsTypedIaCPlugin` and binary path `./cmd/workflow-plugin-gcp` |
| `scripts/workflow-iac-host-conformance.sh` | NEW: copy from AWS v1.0.0, adapt **test name** |

**Critical — conformance script test name:** `scripts/workflow-iac-host-conformance.sh` MUST run:
```bash
WORKFLOW_IAC_HOST_CONFORMANCE=1 GOWORK=off go test ./internal -run TestWorkflowHostConformance_LoadsTypedIaCPlugin -count=1 -v
```
NOT `-run TestWorkflowHostConformance_LoadsLegacyIaCModulePlugin` (which is the AWS test name for the legacy module load path). Using the AWS test name on GCP would silently pass with 0 tests run — the gate must be verified by asserting the test output contains `PASS` and the test name.

**Verification step (mandatory):** After writing `host_conformance_test.go` and `scripts/workflow-iac-host-conformance.sh`, the implementer MUST run the conformance script locally and confirm output contains `--- PASS: TestWorkflowHostConformance_LoadsTypedIaCPlugin` before declaring Phase 4 complete.

### Phase 5 — Add `NewGCPProviderConcrete` constructor (merged into Phase 3)

Note: The original Phase 5 heading was redundant — `NewGCPProviderConcrete()` is part of Phase 3's provider.go changes and is covered there. This phase heading is removed to avoid confusion.

## Compile-time guards

```go
var (
    _ pb.IaCProviderRequiredServer      = (*gcpIaCServer)(nil)
    _ pb.IaCProviderDriftDetectorServer = (*gcpIaCServer)(nil)
    _ pb.ResourceDriverServer           = (*gcpIaCServer)(nil)
)
```

## Wire invariants

- NO `structpb.Struct` on the wire
- Config/outputs cross as `config_json`/`outputs_json` (JSON bytes)
- No typed slices (`[]string` etc.) in `Outputs` map values — all map values are JSON-safe
  scalar types (string, number, bool, nil) or nested maps/slices of those

## Rollback

This is a force-cutover with no compat shim. The current `main.go` in v0.1.4 is a
non-functional stub (prints info, exits) — there is no working plugin to revert to.

**If the cutover fails post-merge:**
1. `git revert <merge-sha>` → new commit restoring v0.1.4 state
2. `GOWORK=off go mod tidy` to restore go.sum to v0.19.2-pinned state
3. Push and retag as v0.1.5 (do not re-use v1.0.0 tag)

Old workflow engine tags (pre-v0.50.0) are permanently incompatible after cutover — engine
consumers must upgrade to v0.51.7+ before installing GCP plugin v1.0.0.

## Assumptions

1. `sdk.ServeIaCPlugin` and `sdk.RegisterAllIaCProviderServices` are present and stable
   in workflow v0.51.7 (confirmed: AWS v1.0.0 used v0.51.7).
2. `wftest` package in workflow v0.51.7 is API-compatible with the existing integration
   tests — `wftest.New`, `wftest.WithYAML`, `wftest.MockStep`, `wftest.RecordStep` all
   present (confirmed: AWS v1.0.0 integration tests use same API at v0.51.7).
   **Verification gate:** implementer must run `GOWORK=off go build ./...` after `go mod tidy`
   and confirm zero errors before writing any Phase 1/2 code. Any wftest API break is a
   BLOCKER that must be resolved (by adapting test calls) before proceeding.
3. `*GCPProvider.DetectDrift` only implements existence-check. `DetectDriftWithSpecs`
   is implemented as a thin delegator.
4. `plugin.contracts.json` is loaded by the engine manager from disk independently of
   gRPC service registration — confirmed via AWS+DO v1.0.1 precedents.
5. The `host_conformance_test.go` can validate the typed-IaC load path without live GCP
   credentials — the plugin binary starts and responds to `Name()` + `Capabilities()` RPCs
   without an initialized GCP session.
6. The v0.19.2 → v0.51.7 go.mod bump will not break `interfaces.*` or
   `plugin/external/proto/*` APIs used by `GCPProvider` — confirmed via AWS v1.0.0
   same-version bump compiling cleanly.
7. Moving `integration_test.go` from `package main_test` at root to `package provider_test`
   at `provider/` requires no test logic changes — the tests use only `wftest` mocks.

## Open questions resolved

- Q: Multi-PR vs single-PR? A: Single-PR force-cutover per precedent and
  `feedback_force_strict_contracts_no_compat`.
- Q: Should `integration_test.go` be deleted or moved? A: Moved to `provider/` —
  the tests exercise the provider layer and are valid. Deleting them would reduce coverage.
- Q: Keep `deploy.go` / `deploy_test.go` in `provider/`? A: Yes — these implement
  Cloud Run deployment strategies (rolling/blue-green/canary) and are independent of
  the plugin entrypoint.
