package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// CloudRunDriver manages Cloud Run services.
type CloudRunDriver struct {
	Client    CloudRunClient
	ProjectID string
	Region    string
}

func (d *CloudRunDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.CreateService(ctx, d.ProjectID, d.Region, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("cloud run create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"service_id": id,
			"region":     d.Region,
			"url":        fmt.Sprintf("https://%s-%s.a.run.app", spec.Name, d.ProjectID),
		},
	}, nil
}

func (d *CloudRunDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("cloud run read: %w", err)
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

func (d *CloudRunDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateService(ctx, d.ProjectID, d.Region, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("cloud run update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *CloudRunDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteService(ctx, d.ProjectID, d.Region, ref.ProviderID); err != nil {
		return fmt.Errorf("cloud run delete: %w", err)
	}
	return nil
}

func (d *CloudRunDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
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

func (d *CloudRunDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	status, _ := info["status"].(string)
	healthy := status == "running" || status == "READY" || status == ""
	return &interfaces.HealthResult{Healthy: healthy, Message: fmt.Sprintf("status: %s", status)}, nil
}

func (d *CloudRunDriver) Scale(ctx context.Context, ref interfaces.ResourceRef, replicas int) (*interfaces.ResourceOutput, error) {
	cfg := map[string]any{"min_instances": replicas, "max_instances": replicas}
	if err := d.Client.UpdateService(ctx, d.ProjectID, d.Region, ref.ProviderID, cfg); err != nil {
		return nil, fmt.Errorf("cloud run scale: %w", err)
	}
	return d.Read(ctx, ref)
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *CloudRunDriver) SensitiveKeys() []string { return nil }
