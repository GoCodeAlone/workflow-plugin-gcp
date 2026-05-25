// Command workflow-plugin-gcp is a workflow engine external plugin that
// provides GCP infrastructure provisioning via the typed IaC gRPC contract.
// It runs as a subprocess and communicates with the host (wfctl) via the
// go-plugin protocol.
//
// As of the strict-contracts force-cutover (workflow v0.51.0+),
// the plugin is served via sdk.ServeIaCPlugin which auto-registers every
// typed pb.IaCProvider*Server interface the underlying *GCPProvider satisfies.
// The legacy sdk.Serve / PluginService InvokeService string-dispatch surface
// has been removed entirely — there is no fallback path.
package main

import (
	_ "embed"

	"github.com/GoCodeAlone/workflow-plugin-gcp/internal"
	"github.com/GoCodeAlone/workflow-plugin-gcp/provider"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// pluginJSON is copied from the repository root by GoReleaser before builds
// and is committed for local builds/tests.
//
//go:embed plugin.json
var pluginJSON []byte

func main() {
	sdk.ServeIaCPlugin(internal.NewIaCServer(), sdk.IaCServeOptions{
		ManifestProvider: sdk.MustEmbedManifest(pluginJSON),
		Modules:          internal.ModuleProviders(),
		BuildVersion:     sdk.ResolveBuildVersion(provider.Version),
	})
}
