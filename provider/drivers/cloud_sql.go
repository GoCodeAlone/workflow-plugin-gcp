package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// CloudSQLDriver manages Cloud SQL instances.
type CloudSQLDriver struct {
	Client    CloudSQLClient
	ProjectID string
	Region    string
}

func (d *CloudSQLDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	cfg := copyConfig(spec.Config)
	cfg["region"] = d.Region
	id, err := d.Client.CreateInstance(ctx, d.ProjectID, cfg)
	if err != nil {
		return nil, fmt.Errorf("cloud sql create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"instance_id":       id,
			"region":            d.Region,
			"connection_string": fmt.Sprintf("%s:%s:%s", d.ProjectID, d.Region, id),
		},
	}, nil
}

func (d *CloudSQLDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetInstance(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("cloud sql read: %w", err)
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

func (d *CloudSQLDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateInstance(ctx, d.ProjectID, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("cloud sql update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *CloudSQLDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteInstance(ctx, d.ProjectID, ref.ProviderID); err != nil {
		return fmt.Errorf("cloud sql delete: %w", err)
	}
	return nil
}

func (d *CloudSQLDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
	result := &interfaces.DiffResult{}
	for k, v := range desired.Config {
		if cv, ok := current.Outputs[k]; ok && fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", v) {
			result.NeedsUpdate = true
			fc := interfaces.FieldChange{Path: k, Old: cv, New: v}
			if k == "engine" || k == "database_version" {
				fc.ForceNew = true
				result.NeedsReplace = true
			}
			result.Changes = append(result.Changes, fc)
		}
	}
	return result, nil
}

func (d *CloudSQLDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	info, err := d.Client.GetInstance(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	status, _ := info["status"].(string)
	healthy := status == "RUNNABLE" || status == "running" || status == ""
	return &interfaces.HealthResult{Healthy: healthy, Message: fmt.Sprintf("status: %s", status)}, nil
}

func (d *CloudSQLDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for cloud sql; update tier via config instead")
}

func copyConfig(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
