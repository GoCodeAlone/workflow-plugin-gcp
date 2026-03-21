package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// ArtifactRegistryDriver manages Artifact Registry repositories.
type ArtifactRegistryDriver struct {
	Client    ArtifactRegistryClient
	ProjectID string
	Location  string
}

func (d *ArtifactRegistryDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.CreateRepository(ctx, d.ProjectID, d.Location, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("artifact registry create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"repository_id": id,
			"location":      d.Location,
			"registry_url":  fmt.Sprintf("%s-docker.pkg.dev/%s/%s", d.Location, d.ProjectID, id),
		},
	}, nil
}

func (d *ArtifactRegistryDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetRepository(ctx, d.ProjectID, d.Location, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("artifact registry read: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       ref.Name,
		Type:       ref.Type,
		ProviderID: ref.ProviderID,
		Status:     "running",
		Outputs:    info,
	}, nil
}

func (d *ArtifactRegistryDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateRepository(ctx, d.ProjectID, d.Location, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("artifact registry update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *ArtifactRegistryDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteRepository(ctx, d.ProjectID, d.Location, ref.ProviderID); err != nil {
		return fmt.Errorf("artifact registry delete: %w", err)
	}
	return nil
}

func (d *ArtifactRegistryDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
	result := &interfaces.DiffResult{}
	for k, v := range desired.Config {
		if cv, ok := current.Outputs[k]; ok && fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", v) {
			result.NeedsUpdate = true
			fc := interfaces.FieldChange{Path: k, Old: cv, New: v}
			if k == "format" {
				fc.ForceNew = true
				result.NeedsReplace = true
			}
			result.Changes = append(result.Changes, fc)
		}
	}
	return result, nil
}

func (d *ArtifactRegistryDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	_, err := d.Client.GetRepository(ctx, d.ProjectID, d.Location, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	return &interfaces.HealthResult{Healthy: true, Message: "repository active"}, nil
}

func (d *ArtifactRegistryDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for artifact registry")
}
