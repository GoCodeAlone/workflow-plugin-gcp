# Changelog

All notable changes to this project will be documented in this file.

The format is loosely based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] — 2026-05-15

This release absorbs the locked B/C/D plan's PR 8 (`gcs` IaCStateBackend +
`gke` cross-process contract) and adds the plan-2 standalone-module
surface (`storage.gcs`, `gcp.credentials`) on top.

### Added

- **`iac.state` `gcs` backend** (`internal/statebackend/gcs.go`,
  `internal/statebackend_server.go`) — plugin-served GCS IaC state store.
  Ported from workflow core's deleted in-core gcs backend. Configure RPC
  delivers the YAML config; lazy store construction via ADC.
- **`gke` cross-process contract** — `provider/drivers/gke.go` realises
  the existing `ResourceDriver` `infra.k8s_cluster` resource type
  (per the locked B/C/D plan's ADR 0037), reading credentials per-Create
  from `ResourceSpec.Config` and dispatching FQN-vs-bare-name parsing in
  one place (`realGKEClient`).
- **`gcp.credentials` standalone module**
  (`internal/modules/gcp_credentials.go`) — optional DRY module that
  registers a `gcp.credentials` block under a name in the process-local
  `credref` registry. Sibling `storage.gcs` modules reference them via
  `credentials_ref:`.
- **`storage.gcs` standalone module**
  (`internal/modules/storage_gcs.go`) — plugin-native GCS storage module
  via `IaCServeOptions.Modules`. Credentials inline (`credentials:`
  sub-block) or `credentials_ref:` a sibling `gcp.credentials` module.
  ADC fallback when neither surface is present.
- **`internal/gcpcreds.BuildGCPOptions`** — in-plugin gcp credential
  helper: inline `ServiceAccountJSON` → `option.WithCredentialsJSON`;
  empty input → no options (ADC fallback). The gcp credential resolvers
  in workflow core are already SDK-free, so this is the entire in-plugin
  auth surface (contrast aws which also re-homes profile + role_arn).
- **`TestPluginJSONCapabilities_ModuleStep_Parity`** — host-conformance
  test asserting plugin.json `capabilities.moduleTypes` /
  `capabilities.stepTypes` exactly match the providers wired into
  `IaCServeOptions`.

### Changed

- **`plugin.json` `version`**: 1.0.0 → 1.1.0 (compatibility-marker minor
  bump for the new module + state-backend capabilities).
- **`plugin.json` `minEngineVersion`**: 0.52.0 → 0.53.0 — requires
  workflow v0.53.0+ for the `IaCServeOptions.Modules` bridge wiring
  (plan-2 PR 1 SDK extension).
- **`plugin.json` `capabilities.moduleTypes`**: adds `gcp.credentials`
  and `storage.gcs` alongside the existing `iac.provider`.
- **`plugin.json` `capabilities.iacStateBackends`**: `["gcs"]` (added in
  the locked-B/C/D commits on this branch).
- **`go.mod`** pins `github.com/GoCodeAlone/workflow v0.53.0`.

### Notes

- Phase-C core PR (workflow plan-2 Task 17/18) deletes in-core
  `iac_state_gcs.go` + `storage_gcs.go` and drops the GCP SDKs from
  workflow's `go.mod`; it is blocked on this release tag.
- Runtime-launch validation transcript:
  `docs/runtime-validation/gcp-plugin-v1.1.0.md`.

## [1.0.0] — earlier

- Typed-IaC migration; baseline GCP provider surface (Cloud Run / GKE /
  Cloud SQL / Memorystore / VPC / LB / DNS / Artifact Registry / API
  Gateway / Firewall / IAM / GCS / SSL Certificate).
