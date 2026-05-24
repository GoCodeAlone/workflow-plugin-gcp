package drivers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/GoCodeAlone/workflow/interfaces"
	"google.golang.org/api/dns/v1"
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
	if err := d.upsertConfiguredRecords(ctx, id, spec.Config); err != nil {
		return nil, err
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
	records, err := d.Client.ListResourceRecordSets(ctx, d.ProjectID, ref.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("dns records read: %w", err)
	}
	outputs := cloudDNSOutputs(info)
	normalizedRecords := cloudDNSRecordOutputs(records, stringProp(info, "dns_name"))
	outputs["records"] = normalizedRecords
	outputs["record_count"] = len(normalizedRecords)
	return &interfaces.ResourceOutput{
		Name:       ref.Name,
		Type:       ref.Type,
		ProviderID: ref.ProviderID,
		Status:     "running",
		Outputs:    outputs,
	}, nil
}

func (d *DNSDriver) Update(ctx context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	if err := d.Client.UpdateManagedZone(ctx, d.ProjectID, ref.ProviderID, spec.Config); err != nil {
		return nil, fmt.Errorf("dns update: %w", err)
	}
	if err := d.upsertConfiguredRecords(ctx, ref.ProviderID, spec.Config); err != nil {
		return nil, err
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

func (d *DNSDriver) upsertConfiguredRecords(ctx context.Context, zoneID string, config map[string]any) error {
	records, err := cloudDNSConfiguredRecords(config)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := d.Client.UpsertResourceRecordSet(ctx, d.ProjectID, zoneID, record); err != nil {
			return fmt.Errorf("dns record upsert %s %s: %w", record.Name, record.Type, err)
		}
	}
	return nil
}

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

func cloudDNSConfiguredRecords(config map[string]any) ([]*dns.ResourceRecordSet, error) {
	rawRecords, ok := config["records"]
	if !ok || rawRecords == nil {
		return nil, nil
	}
	items, ok := rawRecords.([]any)
	if !ok {
		return nil, fmt.Errorf("dns records must be a list")
	}
	zoneName := stringProp(config, "dns_name")
	if zoneName == "" {
		return nil, fmt.Errorf("dns_name is required when records are configured")
	}
	out := make([]*dns.ResourceRecordSet, 0, len(items))
	for i, item := range items {
		recordConfig, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("dns records[%d] must be an object", i)
		}
		recordType := strings.ToUpper(stringProp(recordConfig, "type"))
		if recordType == "" {
			return nil, fmt.Errorf("dns records[%d].type is required", i)
		}
		rrdatas, err := cloudDNSRrdatas(recordType, recordConfig)
		if err != nil {
			return nil, fmt.Errorf("dns records[%d]: %w", i, err)
		}
		out = append(out, &dns.ResourceRecordSet{
			Name:    cloudDNSAbsoluteRecordName(stringProp(recordConfig, "name"), zoneName),
			Type:    recordType,
			Ttl:     int64(configInt(recordConfig, "ttl", 3600)),
			Rrdatas: rrdatas,
		})
	}
	return out, nil
}

func cloudDNSRrdatas(recordType string, config map[string]any) ([]string, error) {
	values := configList(config, "values")
	if len(values) == 0 {
		if value, ok := config["value"]; ok {
			values = []any{value}
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("%s record requires at least one value", recordType)
	}
	switch recordType {
	case "MX":
		out := make([]string, 0, len(values))
		for _, value := range values {
			m, ok := value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("MX values must be objects")
			}
			out = append(out, fmt.Sprintf("%d %s", configInt(m, "preference", 0), stringProp(m, "exchange")))
		}
		return out, nil
	default:
		out := make([]string, 0, len(values))
		for _, value := range values {
			out = append(out, fmt.Sprint(value))
		}
		return out, nil
	}
}

func cloudDNSRecordOutputs(records []*dns.ResourceRecordSet, zoneName string) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		item := map[string]any{
			"name": cloudDNSRelativeRecordName(record.Name, zoneName),
			"type": strings.ToUpper(record.Type),
			"ttl":  record.Ttl,
		}
		switch strings.ToUpper(record.Type) {
		case "MX":
			values := make([]map[string]any, 0, len(record.Rrdatas))
			for _, rrdata := range record.Rrdatas {
				fields := strings.Fields(rrdata)
				if len(fields) < 2 {
					continue
				}
				preference, _ := strconv.Atoi(fields[0])
				values = append(values, map[string]any{
					"preference": preference,
					"exchange":   strings.Join(fields[1:], " "),
				})
			}
			item["values"] = values
		default:
			item["values"] = append([]string(nil), record.Rrdatas...)
		}
		out = append(out, item)
	}
	return out
}

func cloudDNSAbsoluteRecordName(name, zoneName string) string {
	if name == "" || name == "@" {
		return zoneName
	}
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "." + zoneName
}

func cloudDNSRelativeRecordName(name, zoneName string) string {
	if name == zoneName {
		return "@"
	}
	return strings.TrimSuffix(strings.TrimSuffix(name, zoneName), ".")
}

func configList(config map[string]any, key string) []any {
	switch values := config[key].(type) {
	case []any:
		return values
	case []string:
		out := make([]any, 0, len(values))
		for _, value := range values {
			out = append(out, value)
		}
		return out
	default:
		return nil
	}
}

func configInt(config map[string]any, key string, fallback int) int {
	switch value := config[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return fallback
	}
}
