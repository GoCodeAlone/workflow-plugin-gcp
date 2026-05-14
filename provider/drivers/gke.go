package drivers

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// GKEDriver manages GKE clusters.
type GKEDriver struct {
	Client    GKEClient
	ProjectID string
	Location  string
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

func (d *GKEDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.CreateCluster(ctx, d.ProjectID, d.Location, spec.Config)
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
	info, err := d.Client.GetCluster(ctx, d.ProjectID, d.Location, ref.ProviderID)
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
	if err := d.Client.UpdateCluster(ctx, d.ProjectID, d.Location, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("gke update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *GKEDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteCluster(ctx, d.ProjectID, d.Location, ref.ProviderID); err != nil {
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
	info, err := d.Client.GetCluster(ctx, d.ProjectID, d.Location, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	status, _ := info["status"].(string)
	healthy := status == "RUNNING" || status == "running" || status == ""
	return &interfaces.HealthResult{Healthy: healthy, Message: fmt.Sprintf("status: %s", status)}, nil
}

func (d *GKEDriver) Scale(ctx context.Context, ref interfaces.ResourceRef, replicas int) (*interfaces.ResourceOutput, error) {
	cfg := map[string]any{"node_count": replicas}
	if err := d.Client.UpdateCluster(ctx, d.ProjectID, d.Location, ref.ProviderID, cfg); err != nil {
		return nil, fmt.Errorf("gke scale: %w", err)
	}
	return d.Read(ctx, ref)
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *GKEDriver) SensitiveKeys() []string { return nil }
