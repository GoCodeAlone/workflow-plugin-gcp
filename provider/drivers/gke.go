package drivers

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoCodeAlone/workflow/interfaces"
	"google.golang.org/api/option"
)

// Per-Create credential keys read from ResourceSpec.config_json. These MUST
// match the workflow-core grpcKubernetesBackend host adapter's k8sConfigKey*
// constants (module/platform_kubernetes_grpc.go). Per ADR 0037 the host adapter
// owns the key contract; this plugin conforms to it.
const (
	gkeConfigKeyProjectID          = "project_id"
	gkeConfigKeyServiceAccountJSON = "service_account_json" //nolint:gosec // G101: config map key name, not a credential
)

// GKEDriver manages GKE clusters.
//
// Per ADR 0037 + workflow-core Task 25, credentials travel per-Create in
// ResourceSpec.config_json under gkeConfigKey* keys: Create reads them and
// builds a per-call GKE client via PerCallClientFactory. Read/Delete have no
// per-request config (ResourceRef carries no config_json), so they fall back to
// the Initialize-time Client — that fallback is the engine's responsibility to
// wire with usable credentials when the cross-process gke flow drives Read/
// Delete without a prior in-process Create. See team-lead's "option (a)"
// ruling.
type GKEDriver struct {
	Client    GKEClient
	ProjectID string
	Location  string
	// PerCallClientFactory builds a per-call GKEClient from a service-account
	// JSON payload. Defaults (nil) to a real google.golang.org/api/container
	// client built via option.WithCredentialsJSON. Tests override it to inject
	// a mock client.
	PerCallClientFactory func(ctx context.Context, saJSON string) (GKEClient, error)
}

// resolveCreate selects the GKE client + project for a Create RPC. If the
// caller supplies service_account_json in spec.Config, a per-call client is
// built via PerCallClientFactory (the ADR-0037 canonical seam). Otherwise the
// Initialize-time Client is used. project_id in spec.Config likewise overrides
// the Initialize-time ProjectID.
func (d *GKEDriver) resolveCreate(ctx context.Context, spec interfaces.ResourceSpec) (GKEClient, string, error) {
	project := d.ProjectID
	if p, ok := spec.Config[gkeConfigKeyProjectID].(string); ok && p != "" {
		project = p
	}
	saJSON, _ := spec.Config[gkeConfigKeyServiceAccountJSON].(string)
	if saJSON == "" {
		return d.Client, project, nil
	}
	factory := d.PerCallClientFactory
	if factory == nil {
		factory = func(ctx context.Context, sa string) (GKEClient, error) {
			return NewRealGKEClient(ctx, option.WithCredentialsJSON([]byte(sa)))
		}
	}
	client, err := factory(ctx, saJSON)
	if err != nil {
		return nil, "", fmt.Errorf("build per-call gke client: %w", err)
	}
	return client, project, nil
}

// clusterNameFromSpec resolves the GKE cluster name from the spec — the
// `name` config key, falling back to the resource name. Mirrors realGKEClient's
// CreateCluster name resolution.
func clusterNameFromSpec(spec interfaces.ResourceSpec) string {
	if n, ok := spec.Config["name"].(string); ok && n != "" {
		return n
	}
	return spec.Name
}

// isAlreadyExists reports whether a GKE Create error means the cluster already
// exists — the cross-process path must swallow it as success, exactly as the
// in-core gkeBackend did (ADR 0037 Consequences).
func isAlreadyExists(err error) bool {
	s := err.Error()
	return strings.Contains(s, "AlreadyExists") ||
		strings.Contains(s, "ALREADY_EXISTS") ||
		strings.Contains(s, "Already Exists")
}

// isNotFound reports whether a GKE Delete error means the cluster is already
// gone — Delete must swallow it as success, mirroring the in-core gkeBackend.
func isNotFound(err error) bool {
	s := err.Error()
	return strings.Contains(s, "NotFound") ||
		strings.Contains(s, "NOT_FOUND") ||
		strings.Contains(s, "notFound")
}

// gkeResourceFromRef parses a ResourceRef into (project, location, clusterName)
// — accepting two ProviderID forms:
//
//   - Fully-qualified: "projects/<p>/locations/<l>/clusters/<n>" — the form
//     workflow-core's grpcKubernetesBackend host adapter writes after Task 25's
//     fully-qualified-ProviderId fix (GKE names alone are not globally unique).
//   - Bare cluster name "<n>" or empty — falls back to d.ProjectID / d.Location
//     for the missing components (the legacy in-process form + a defensive
//     default for callers that haven't migrated).
//
// The realGKEClient's GetCluster / DeleteCluster / UpdateCluster expect bare
// (projectID, location, clusterID) components and wrap them into the FQN
// themselves — so the driver must NOT pass an FQN straight through (that would
// double-wrap into a malformed path).
func (d *GKEDriver) gkeResourceFromRef(ref interfaces.ResourceRef) (project, location, clusterName string) {
	pid := ref.ProviderID
	if strings.HasPrefix(pid, "projects/") {
		parts := strings.Split(pid, "/")
		if len(parts) == 6 && parts[0] == "projects" && parts[2] == "locations" && parts[4] == "clusters" {
			return parts[1], parts[3], parts[5]
		}
	}
	return d.ProjectID, d.Location, pid
}

func (d *GKEDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	client, project, err := d.resolveCreate(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("gke create: %w", err)
	}
	id, err := client.CreateCluster(ctx, project, d.Location, spec.Config)
	if err != nil {
		if isAlreadyExists(err) {
			// Idempotent: a pre-existing cluster is success — mirrors the
			// in-core gkeBackend which swallowed ALREADY_EXISTS. Per ADR 0037.
			name := clusterNameFromSpec(spec)
			return &interfaces.ResourceOutput{
				Name:       spec.Name,
				Type:       spec.Type,
				ProviderID: name,
				Status:     "running",
				Outputs: map[string]any{
					"cluster_id": name,
					"location":   d.Location,
					"status":     "running",
				},
			}, nil
		}
		return nil, fmt.Errorf("gke create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"cluster_id": id,
			"location":   d.Location,
			"endpoint":   fmt.Sprintf("https://%s.%s.gke.io", spec.Name, d.Location),
		},
	}, nil
}

func (d *GKEDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	project, location, clusterName := d.gkeResourceFromRef(ref)
	info, err := d.Client.GetCluster(ctx, project, location, clusterName)
	if err != nil {
		return nil, fmt.Errorf("gke read: %w", err)
	}
	status, _ := info["status"].(string)
	if status == "" {
		status = "running"
	}
	return &interfaces.ResourceOutput{
		Name:       ref.Name,
		Type:       ref.Type,
		ProviderID: ref.ProviderID,
		Status:     status,
		Outputs:    info,
	}, nil
}

func (d *GKEDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	project, location, clusterName := d.gkeResourceFromRef(ref)
	if err := d.Client.UpdateCluster(ctx, project, location, clusterName, spec.Config); err != nil {
		return nil, fmt.Errorf("gke update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *GKEDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	project, location, clusterName := d.gkeResourceFromRef(ref)
	if err := d.Client.DeleteCluster(ctx, project, location, clusterName); err != nil {
		if isNotFound(err) {
			// Idempotent: an already-gone cluster is success — mirrors the
			// in-core gkeBackend which swallowed NOT_FOUND. Per ADR 0037.
			return nil
		}
		return fmt.Errorf("gke delete: %w", err)
	}
	return nil
}

func (d *GKEDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
	result := &interfaces.DiffResult{}
	for k, v := range desired.Config {
		if cv, ok := current.Outputs[k]; ok && fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", v) {
			result.NeedsUpdate = true
			result.Changes = append(result.Changes, interfaces.FieldChange{
				Path: k, Old: cv, New: v,
			})
		}
	}
	return result, nil
}

func (d *GKEDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	project, location, clusterName := d.gkeResourceFromRef(ref)
	info, err := d.Client.GetCluster(ctx, project, location, clusterName)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	status, _ := info["status"].(string)
	healthy := status == "RUNNING" || status == "running" || status == ""
	return &interfaces.HealthResult{Healthy: healthy, Message: fmt.Sprintf("status: %s", status)}, nil
}

func (d *GKEDriver) Scale(ctx context.Context, ref interfaces.ResourceRef, replicas int) (*interfaces.ResourceOutput, error) {
	project, location, clusterName := d.gkeResourceFromRef(ref)
	cfg := map[string]any{"node_count": replicas}
	if err := d.Client.UpdateCluster(ctx, project, location, clusterName, cfg); err != nil {
		return nil, fmt.Errorf("gke scale: %w", err)
	}
	return d.Read(ctx, ref)
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *GKEDriver) SensitiveKeys() []string { return nil }
