package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// FirewallDriver manages GCP VPC firewall rules.
type FirewallDriver struct {
	Client    FirewallClient
	ProjectID string
}

func (d *FirewallDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.CreateRule(ctx, d.ProjectID, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("firewall create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"rule_id":   id,
			"direction": spec.Config["direction"],
		},
	}, nil
}

func (d *FirewallDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetRule(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("firewall read: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       ref.Name,
		Type:       ref.Type,
		ProviderID: ref.ProviderID,
		Status:     "running",
		Outputs:    info,
	}, nil
}

func (d *FirewallDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateRule(ctx, d.ProjectID, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("firewall update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *FirewallDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteRule(ctx, d.ProjectID, ref.ProviderID); err != nil {
		return fmt.Errorf("firewall delete: %w", err)
	}
	return nil
}

func (d *FirewallDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
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

func (d *FirewallDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	_, err := d.Client.GetRule(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	return &interfaces.HealthResult{Healthy: true, Message: "firewall rule active"}, nil
}

func (d *FirewallDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for firewall rules")
}
