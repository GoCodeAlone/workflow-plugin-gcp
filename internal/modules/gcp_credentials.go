// Package modules implements the gcp plugin's standalone-module surface:
// plugin-native modules (gcp.credentials, storage.gcs, etc.) the host
// constructs via the IaC plugin's CreateModule path (plan-2 PR 1 wired
// sdk.IaCServeOptions.Modules into the iacGRPCPlugin bridge).
//
// gcp.credentials is the optional DRY module: a config declares
// credentials once under a module name; sibling modules
// (storage.gcs, step.gcs_*, etc.) reference them via `credentials_ref:`
// instead of repeating the inline `credentials:` block.
package modules

import (
	"context"

	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/credref"
	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/gcpcreds"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
)

// GCPCredentialsProvider implements sdk.ModuleProvider for the
// "gcp.credentials" standalone-module type.
type GCPCredentialsProvider struct{}

// NewGCPCredentialsProvider returns a fresh provider. Construction is
// trivial — kept as a named constructor for symmetry with the other
// plugin Providers + so callers can wire it into IaCServeOptions.Modules
// without referring to the (otherwise empty) struct literal directly.
func NewGCPCredentialsProvider() *GCPCredentialsProvider {
	return &GCPCredentialsProvider{}
}

// ModuleTypes reports the single module type this Provider serves.
func (p *GCPCredentialsProvider) ModuleTypes() []string {
	return []string{"gcp.credentials"}
}

// CreateModule parses the YAML `credentials:` block (plus an optional
// top-level `projectId` override) into a gcpcreds.CredInput and registers
// it in the process-local credref registry under the module name. A
// duplicate name fails loudly — credentials_ref names must be unique.
func (p *GCPCredentialsProvider) CreateModule(_, name string, config map[string]any) (sdk.ModuleInstance, error) {
	credsMap, _ := config["credentials"].(map[string]any)
	c := gcpcreds.CredInput{
		ProjectID: stringField(credsMap, "projectId"),
	}
	if sa := stringField(credsMap, "serviceAccountJson"); sa != "" {
		c.ServiceAccountJSON = []byte(sa)
	}
	// Top-level projectId is a back-compat affordance for configs that
	// hoist the project ID alongside other module config rather than
	// nesting it under credentials:. Inline `credentials.projectId`
	// takes priority.
	if c.ProjectID == "" {
		c.ProjectID = stringField(config, "projectId")
	}
	if err := credref.Register(name, c); err != nil {
		return nil, err
	}
	return &gcpCredentialsInstance{name: name}, nil
}

// gcpCredentialsInstance is the lifecycle-only module instance the
// gcp.credentials Provider returns. The actual credential effect was
// the credref.Register call in CreateModule; the instance itself has
// no runtime behavior.
type gcpCredentialsInstance struct {
	name string
}

func (m *gcpCredentialsInstance) Init() error                  { return nil }
func (m *gcpCredentialsInstance) Start(_ context.Context) error { return nil }
func (m *gcpCredentialsInstance) Stop(_ context.Context) error  { return nil }

func stringField(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	v, _ := m[k].(string)
	return v
}
