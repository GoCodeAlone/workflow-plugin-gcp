package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// SSLCertificateDriver manages GCP SSL certificates.
type SSLCertificateDriver struct {
	Client    SSLCertificateClient
	ProjectID string
}

func (d *SSLCertificateDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.CreateCertificate(ctx, d.ProjectID, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("ssl certificate create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: map[string]any{
			"certificate_id": id,
			"managed":        true,
		},
	}, nil
}

func (d *SSLCertificateDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetCertificate(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("ssl certificate read: %w", err)
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

func (d *SSLCertificateDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateCertificate(ctx, d.ProjectID, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("ssl certificate update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *SSLCertificateDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteCertificate(ctx, d.ProjectID, ref.ProviderID); err != nil {
		return fmt.Errorf("ssl certificate delete: %w", err)
	}
	return nil
}

func (d *SSLCertificateDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
	result := &interfaces.DiffResult{}
	for k, v := range desired.Config {
		if cv, ok := current.Outputs[k]; ok && fmt.Sprintf("%v", cv) != fmt.Sprintf("%v", v) {
			result.NeedsUpdate = true
			fc := interfaces.FieldChange{Path: k, Old: cv, New: v}
			if k == "domains" {
				fc.ForceNew = true
				result.NeedsReplace = true
			}
			result.Changes = append(result.Changes, fc)
		}
	}
	return result, nil
}

func (d *SSLCertificateDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	info, err := d.Client.GetCertificate(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	status, _ := info["status"].(string)
	healthy := status == "ACTIVE" || status == "running" || status == ""
	return &interfaces.HealthResult{Healthy: healthy, Message: fmt.Sprintf("status: %s", status)}, nil
}

func (d *SSLCertificateDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for ssl certificates")
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *SSLCertificateDriver) SensitiveKeys() []string { return nil }
