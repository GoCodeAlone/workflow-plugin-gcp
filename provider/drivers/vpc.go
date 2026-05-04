package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// VPCDriver manages GCP VPC networks and subnets.
// GCP VPCs are global; subnets are regional.
type VPCDriver struct {
	Client    VPCClient
	ProjectID string
	Region    string
}

func (d *VPCDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	networkID, err := d.Client.CreateNetwork(ctx, d.ProjectID, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("vpc create network: %w", err)
	}

	// Create a regional subnet within the VPC.
	subnetCfg := map[string]any{
		"network":   networkID,
		"ip_range":  spec.Config["subnet_cidr"],
		"name":      fmt.Sprintf("%s-subnet", spec.Name),
	}
	subnetID, err := d.Client.CreateSubnet(ctx, d.ProjectID, d.Region, subnetCfg)
	if err != nil {
		return nil, fmt.Errorf("vpc create subnet: %w", err)
	}

	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: networkID,
		Status:     "running",
		Outputs: map[string]any{
			"network_id": networkID,
			"subnet_id":  subnetID,
			"region":     d.Region,
		},
	}, nil
}

func (d *VPCDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetNetwork(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("vpc read: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       ref.Name,
		Type:       ref.Type,
		ProviderID: ref.ProviderID,
		Status:     "running",
		Outputs:    info,
	}, nil
}

func (d *VPCDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	// VPC networks are mostly immutable; subnets can be updated.
	if subnetID, ok := spec.Config["subnet_id"].(string); ok {
		if err := d.Client.DeleteSubnet(ctx, d.ProjectID, d.Region, subnetID); err != nil {
			return nil, fmt.Errorf("vpc update delete old subnet: %w", err)
		}
		subnetCfg := map[string]any{
			"network":  ref.ProviderID,
			"ip_range": spec.Config["subnet_cidr"],
			"name":     fmt.Sprintf("%s-subnet", spec.Name),
		}
		newSubnetID, err := d.Client.CreateSubnet(ctx, d.ProjectID, d.Region, subnetCfg)
		if err != nil {
			return nil, fmt.Errorf("vpc update create new subnet: %w", err)
		}
		spec.Config["subnet_id"] = newSubnetID
	}
	return d.Read(ctx, ref)
}

func (d *VPCDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	// Delete subnets first, then the network.
	info, err := d.Client.GetNetwork(ctx, d.ProjectID, ref.ProviderID)
	if err == nil {
		if subnetID, ok := info["subnet_id"].(string); ok {
			_ = d.Client.DeleteSubnet(ctx, d.ProjectID, d.Region, subnetID)
		}
	}
	if err := d.Client.DeleteNetwork(ctx, d.ProjectID, ref.ProviderID); err != nil {
		return fmt.Errorf("vpc delete: %w", err)
	}
	return nil
}

func (d *VPCDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
	result := &interfaces.DiffResult{}
	for k, v := range desired.Config {
		if cv, ok := current.Outputs[k]; ok && fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", v) {
			result.NeedsUpdate = true
			fc := interfaces.FieldChange{Path: k, Old: cv, New: v}
			if k == "subnet_cidr" {
				fc.ForceNew = true
				result.NeedsReplace = true
			}
			result.Changes = append(result.Changes, fc)
		}
	}
	return result, nil
}

func (d *VPCDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	_, err := d.Client.GetNetwork(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	return &interfaces.HealthResult{Healthy: true, Message: "vpc network exists"}, nil
}

func (d *VPCDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for vpc")
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *VPCDriver) SensitiveKeys() []string { return nil }
