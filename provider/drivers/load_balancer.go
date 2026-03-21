package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// LoadBalancerDriver manages GCP load balancers.
type LoadBalancerDriver struct {
	Client    LoadBalancerClient
	ProjectID string
	Region    string
}

func (d *LoadBalancerDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.Create(ctx, d.ProjectID, d.Region, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("lb create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"lb_id":  id,
			"region": d.Region,
			"ip":     fmt.Sprintf("34.%s.0.1", spec.Name),
		},
	}, nil
}

func (d *LoadBalancerDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.Get(ctx, d.ProjectID, d.Region, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("lb read: %w", err)
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

func (d *LoadBalancerDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.Update(ctx, d.ProjectID, d.Region, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("lb update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *LoadBalancerDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.Delete(ctx, d.ProjectID, d.Region, ref.ProviderID); err != nil {
		return fmt.Errorf("lb delete: %w", err)
	}
	return nil
}

func (d *LoadBalancerDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
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

func (d *LoadBalancerDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	_, err := d.Client.Get(ctx, d.ProjectID, d.Region, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	return &interfaces.HealthResult{Healthy: true, Message: "load balancer active"}, nil
}

func (d *LoadBalancerDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for load balancer")
}
