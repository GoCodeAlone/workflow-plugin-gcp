package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// MemorystoreDriver manages Memorystore Redis instances.
type MemorystoreDriver struct {
	Client    MemorystoreClient
	ProjectID string
	Region    string
}

func (d *MemorystoreDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.CreateInstance(ctx, d.ProjectID, d.Region, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("memorystore create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"instance_id": id,
			"region":      d.Region,
			"host":        fmt.Sprintf("%s.redis.%s.gcp", id, d.Region),
			"port":        6379,
		},
	}, nil
}

func (d *MemorystoreDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetInstance(ctx, d.ProjectID, d.Region, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("memorystore read: %w", err)
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

func (d *MemorystoreDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateInstance(ctx, d.ProjectID, d.Region, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("memorystore update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *MemorystoreDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteInstance(ctx, d.ProjectID, d.Region, ref.ProviderID); err != nil {
		return fmt.Errorf("memorystore delete: %w", err)
	}
	return nil
}

func (d *MemorystoreDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
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

func (d *MemorystoreDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	info, err := d.Client.GetInstance(ctx, d.ProjectID, d.Region, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	status, _ := info["status"].(string)
	healthy := status == "READY" || status == "running" || status == ""
	return &interfaces.HealthResult{Healthy: healthy, Message: fmt.Sprintf("status: %s", status)}, nil
}

func (d *MemorystoreDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for memorystore; update memory_size_gb via config instead")
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *MemorystoreDriver) SensitiveKeys() []string { return nil }
