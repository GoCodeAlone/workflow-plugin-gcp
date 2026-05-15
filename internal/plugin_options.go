// plugin_options.go — single source of truth for the providers wired into
// sdk.IaCServeOptions. main.go and the host-conformance parity test both
// consume these helpers so plugin.json declarations and the running plugin's
// surface cannot drift.
package internal

import (
	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/modules"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// ModuleProviders returns the type-name → sdk.ModuleProvider map the plugin
// surfaces via IaCServeOptions.Modules.
//
// The map keys MUST equal plugin.json `capabilities.moduleTypes` (modulo
// the implicit "iac.provider", served via the IaC contract surface);
// the parity test in host_conformance_test.go enforces that invariant.
func ModuleProviders() map[string]sdk.ModuleProvider {
	return map[string]sdk.ModuleProvider{
		"gcp.credentials": modules.NewGCPCredentialsProvider(),
		"storage.gcs":     modules.NewGCSStorageProvider(),
	}
}

// StepProviders returns an empty map — the gcp plugin currently serves no
// pipeline steps. Kept for symmetry with the aws plugin so main.go's
// IaCServeOptions wiring is uniform.
func StepProviders() map[string]sdk.StepProvider {
	return nil
}
