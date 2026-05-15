// Package gcpcreds provides the in-plugin GCP credential helper. It converts
// a CredInput (parsed from a `credentials:` block, or resolved from the
// `credref` registry) into a []option.ClientOption suitable for constructing
// any cloud.google.com/go/* client.
//
// The gcp credential resolvers in workflow core (module/cloud_account_gcp.go)
// are already SDK-free, so this helper is the entire in-plugin auth surface —
// no broader resolver-replacement is needed (contrast the aws plugin, which
// also reimplements profile + role_arn resolution).
package gcpcreds

import "google.golang.org/api/option"

// CredInput is the parsed-config shape the plugin's credential resolvers
// produce. It mirrors the fields workflow core's module.CloudCredentials
// surfaces for GCP. Both fields are optional; an empty CredInput causes the
// SDK's Application Default Credentials chain to apply.
type CredInput struct {
	// ServiceAccountJSON is an inline GCP service-account JSON key. When
	// non-empty it is passed straight to option.WithCredentialsJSON.
	ServiceAccountJSON []byte
	// ProjectID is the GCP project ID. Carried for downstream consumers
	// (e.g. quota-project wiring); BuildGCPOptions does NOT synthesize a
	// credential option from ProjectID alone.
	ProjectID string
}

// BuildGCPOptions returns the []option.ClientOption a GCP SDK client should
// be constructed with for the given CredInput.
//
// Rules:
//   - ServiceAccountJSON non-empty → option.WithCredentialsJSON.
//   - Empty CredInput (or ProjectID-only) → no credential option emitted;
//     the SDK's Application Default Credentials chain applies.
func BuildGCPOptions(c CredInput) []option.ClientOption {
	var opts []option.ClientOption
	if len(c.ServiceAccountJSON) > 0 {
		opts = append(opts, option.WithCredentialsJSON(c.ServiceAccountJSON))
	}
	return opts
}
