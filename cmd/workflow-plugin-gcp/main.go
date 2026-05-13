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
	"github.com/GoCodeAlone/workflow-plugin-gcp/internal"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

func main() {
	sdk.ServeIaCPlugin(internal.NewIaCServer(), sdk.IaCServeOptions{})
}
