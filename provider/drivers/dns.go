package drivers

import (
	"context"
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

// DNSDriver manages Cloud DNS managed zones.
type DNSDriver struct {
	Client    DNSClient
	ProjectID string
}

func (d *DNSDriver) Create(ctx context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	id, err := d.Client.CreateManagedZone(ctx, d.ProjectID, spec.Config)
	if err != nil {
		return nil, fmt.Errorf("dns create: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       spec.Name,
		Type:       spec.Type,
		ProviderID: id,
		Status:     "running",
		Outputs: cloudDNSOutputs(map[string]any{
			"zone_id":      id,
			"domain":       stringProp(spec.Config, "dns_name"),
			"dns_name":     stringProp(spec.Config, "dns_name"),
			"name_servers": []string{"ns-cloud-a1.googledomains.com"},
		}),
	}, nil
}

func (d *DNSDriver) Read(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	info, err := d.Client.GetManagedZone(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("dns read: %w", err)
	}
	return &interfaces.ResourceOutput{
		Name:       ref.Name,
		Type:       ref.Type,
		ProviderID: ref.ProviderID,
		Status:     "running",
		Outputs:    cloudDNSOutputs(info),
	}, nil
}

func (d *DNSDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateManagedZone(ctx, d.ProjectID, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("dns update: %w", err)
	}
	return d.Read(ctx, ref)
}

func (d *DNSDriver) Delete(ctx context.Context, ref interfaces.ResourceRef) error {
	if err := d.Client.DeleteManagedZone(ctx, d.ProjectID, ref.ProviderID); err != nil {
		return fmt.Errorf("dns delete: %w", err)
	}
	return nil
}

func (d *DNSDriver) Diff(_ context.Context, desired interfaces.ResourceSpec, current *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
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

func (d *DNSDriver) HealthCheck(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	_, err := d.Client.GetManagedZone(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return &interfaces.HealthResult{Healthy: false, Message: err.Error()}, nil
	}
	return &interfaces.HealthResult{Healthy: true, Message: "dns zone active"}, nil
}

func (d *DNSDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, fmt.Errorf("scale not supported for dns")
}

// SensitiveKeys returns output keys whose values should be masked in logs and plan output.
func (d *DNSDriver) SensitiveKeys() []string { return nil }

func cloudDNSOutputs(info map[string]any) map[string]any {
	outputs := make(map[string]any, len(info)+2)
	for key, value := range info {
		outputs[key] = value
	}
	if _, ok := outputs["domain"]; !ok {
		if dnsName, ok := outputs["dns_name"].(string); ok {
			outputs["domain"] = dnsName
		}
	}
	nameServers := stringSlice(outputs["name_servers"])
	outputs["authority"] = map[string]any{
		"role":         "target_authoritative_dns",
		"dns_host":     "Cloud DNS",
		"name_servers": nameServers,
	}
	return outputs
}

func stringProp(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
