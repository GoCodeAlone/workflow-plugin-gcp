package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// APIGatewayDriver manages GCP API Gateway resources.
type APIGatewayDriver struct {
	Client    APIGatewayClient
	ProjectID string
	Region    string
}

func (d *APIGatewayDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.CreateGateway(ctx, d.ProjectID, d.Region, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("api gateway create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"gateway_id": id,
			"region":     d.Region,
			"endpoint":   fmt.Sprintf("https://%s-%s.gateway.dev", id, d.ProjectID),
		},
	}, nil
}

func (d *APIGatewayDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetGateway(ctx, d.ProjectID, d.Region, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("api gateway read: %w", err)
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

func (d *APIGatewayDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateGateway(ctx, d.ProjectID, d.Region, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("api gateway update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *APIGatewayDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteGateway(ctx, d.ProjectID, d.Region, ref.ProviderID); err != nil {
		return fmt.Errorf("api gateway delete: %w", err)
	}
	return nil
}

func (d *APIGatewayDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
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

func (d *APIGatewayDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	_, err := d.Client.GetGateway(ctx, d.ProjectID, d.Region, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	return &interfaces.HealthResult{Healthy: true, Message: "api gateway active"}, nil
}

func (d *APIGatewayDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for api gateway")
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *APIGatewayDriver) SensitiveKeys() []string { return nil }
