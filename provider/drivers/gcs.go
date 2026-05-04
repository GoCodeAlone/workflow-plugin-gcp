package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// GCSDriver manages Cloud Storage buckets.
type GCSDriver struct {
	Client    GCSClient
	ProjectID string
}

func (d *GCSDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	cfg := copyConfig(spec.Config)
	cfg["project"] = d.ProjectID
	id, err := d.Client.CreateBucket(ctx, d.ProjectID, cfg)
	if err != nil {
		return nil, fmt.Errorf("gcs create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"bucket_id": id,
			"url":       fmt.Sprintf("gs://%s", id),
		},
	}, nil
}

func (d *GCSDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetBucket(ctx, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("gcs read: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       ref.Name,
		Type:       ref.Type,
		ProviderID: ref.ProviderID,
		Status:     "running",
		Outputs:    info,
	}, nil
}

func (d *GCSDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateBucket(ctx, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("gcs update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *GCSDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteBucket(ctx, ref.ProviderID); err != nil {
		return fmt.Errorf("gcs delete: %w", err)
	}
	return nil
}

func (d *GCSDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
	result := &interfaces.DiffResult{}
	for k, v := range desired.Config {
		if cv, ok := current.Outputs[k]; ok && fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", v) {
			result.NeedsUpdate = true
			fc := interfaces.FieldChange{Path: k, Old: cv, New: v}
			if k == "location" {
				fc.ForceNew = true
				result.NeedsReplace = true
			}
			result.Changes = append(result.Changes, fc)
		}
	}
	return result, nil
}

func (d *GCSDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	_, err := d.Client.GetBucket(ctx, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	return &interfaces.HealthResult{Healthy: true, Message: "bucket exists"}, nil
}

func (d *GCSDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for cloud storage")
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *GCSDriver) SensitiveKeys() []string { return nil }
