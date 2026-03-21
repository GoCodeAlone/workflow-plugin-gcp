package provider

import (
	"fmt"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type sizingEntry struct {
	InstanceType string
	Specs        map[string]any
}

var containerServiceSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "cloud-run-xs", Specs: map[string]any{"cpu": "250m", "memory": "512Mi"}},
	interfaces.SizeS:  {InstanceType: "cloud-run-s", Specs: map[string]any{"cpu": "1000m", "memory": "2Gi"}},
	interfaces.SizeM:  {InstanceType: "cloud-run-m", Specs: map[string]any{"cpu": "2000m", "memory": "4Gi"}},
	interfaces.SizeL:  {InstanceType: "cloud-run-l", Specs: map[string]any{"cpu": "4000m", "memory": "8Gi"}},
	interfaces.SizeXL: {InstanceType: "cloud-run-xl", Specs: map[string]any{"cpu": "8000m", "memory": "16Gi"}},
}

var k8sClusterSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "e2-micro", Specs: map[string]any{"machine_type": "e2-micro", "node_count": 1}},
	interfaces.SizeS:  {InstanceType: "e2-small", Specs: map[string]any{"machine_type": "e2-small", "node_count": 2}},
	interfaces.SizeM:  {InstanceType: "n2-standard-2", Specs: map[string]any{"machine_type": "n2-standard-2", "node_count": 3}},
	interfaces.SizeL:  {InstanceType: "n2-standard-4", Specs: map[string]any{"machine_type": "n2-standard-4", "node_count": 3}},
	interfaces.SizeXL: {InstanceType: "n2-standard-8", Specs: map[string]any{"machine_type": "n2-standard-8", "node_count": 5}},
}

var databaseSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "db-f1-micro", Specs: map[string]any{"tier": "db-f1-micro", "disk_size_gb": 10}},
	interfaces.SizeS:  {InstanceType: "db-g1-small", Specs: map[string]any{"tier": "db-g1-small", "disk_size_gb": 50}},
	interfaces.SizeM:  {InstanceType: "db-custom-2-8192", Specs: map[string]any{"tier": "db-custom-2-8192", "disk_size_gb": 100}},
	interfaces.SizeL:  {InstanceType: "db-n1-standard-4", Specs: map[string]any{"tier": "db-n1-standard-4", "disk_size_gb": 500}},
	interfaces.SizeXL: {InstanceType: "db-n1-standard-8", Specs: map[string]any{"tier": "db-n1-standard-8", "disk_size_gb": 1000}},
}

var cacheSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "BASIC-1", Specs: map[string]any{"tier": "BASIC", "memory_size_gb": 1}},
	interfaces.SizeS:  {InstanceType: "BASIC-5", Specs: map[string]any{"tier": "BASIC", "memory_size_gb": 5}},
	interfaces.SizeM:  {InstanceType: "STANDARD_HA-10", Specs: map[string]any{"tier": "STANDARD_HA", "memory_size_gb": 10}},
	interfaces.SizeL:  {InstanceType: "STANDARD_HA-30", Specs: map[string]any{"tier": "STANDARD_HA", "memory_size_gb": 30}},
	interfaces.SizeXL: {InstanceType: "STANDARD_HA-100", Specs: map[string]any{"tier": "STANDARD_HA", "memory_size_gb": 100}},
}

var vpcSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "vpc-auto", Specs: map[string]any{"routing_mode": "REGIONAL"}},
	interfaces.SizeS:  {InstanceType: "vpc-auto", Specs: map[string]any{"routing_mode": "REGIONAL"}},
	interfaces.SizeM:  {InstanceType: "vpc-auto", Specs: map[string]any{"routing_mode": "GLOBAL"}},
	interfaces.SizeL:  {InstanceType: "vpc-auto", Specs: map[string]any{"routing_mode": "GLOBAL"}},
	interfaces.SizeXL: {InstanceType: "vpc-auto", Specs: map[string]any{"routing_mode": "GLOBAL"}},
}

var loadBalancerSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "EXTERNAL", Specs: map[string]any{"scheme": "EXTERNAL"}},
	interfaces.SizeS:  {InstanceType: "EXTERNAL", Specs: map[string]any{"scheme": "EXTERNAL"}},
	interfaces.SizeM:  {InstanceType: "EXTERNAL_MANAGED", Specs: map[string]any{"scheme": "EXTERNAL_MANAGED"}},
	interfaces.SizeL:  {InstanceType: "EXTERNAL_MANAGED", Specs: map[string]any{"scheme": "EXTERNAL_MANAGED"}},
	interfaces.SizeXL: {InstanceType: "EXTERNAL_MANAGED", Specs: map[string]any{"scheme": "EXTERNAL_MANAGED", "enable_cdn": true}},
}

var dnsSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "public", Specs: map[string]any{"visibility": "public"}},
	interfaces.SizeS:  {InstanceType: "public", Specs: map[string]any{"visibility": "public"}},
	interfaces.SizeM:  {InstanceType: "public", Specs: map[string]any{"visibility": "public"}},
	interfaces.SizeL:  {InstanceType: "private", Specs: map[string]any{"visibility": "private"}},
	interfaces.SizeXL: {InstanceType: "private", Specs: map[string]any{"visibility": "private"}},
}

var registrySizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "DOCKER", Specs: map[string]any{"format": "DOCKER"}},
	interfaces.SizeS:  {InstanceType: "DOCKER", Specs: map[string]any{"format": "DOCKER"}},
	interfaces.SizeM:  {InstanceType: "DOCKER", Specs: map[string]any{"format": "DOCKER"}},
	interfaces.SizeL:  {InstanceType: "DOCKER", Specs: map[string]any{"format": "DOCKER", "immutable_tags": true}},
	interfaces.SizeXL: {InstanceType: "DOCKER", Specs: map[string]any{"format": "DOCKER", "immutable_tags": true}},
}

var apiGatewaySizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "gateway", Specs: map[string]any{}},
	interfaces.SizeS:  {InstanceType: "gateway", Specs: map[string]any{}},
	interfaces.SizeM:  {InstanceType: "gateway", Specs: map[string]any{}},
	interfaces.SizeL:  {InstanceType: "gateway", Specs: map[string]any{}},
	interfaces.SizeXL: {InstanceType: "gateway", Specs: map[string]any{}},
}

var firewallSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "firewall-rule", Specs: map[string]any{}},
	interfaces.SizeS:  {InstanceType: "firewall-rule", Specs: map[string]any{}},
	interfaces.SizeM:  {InstanceType: "firewall-rule", Specs: map[string]any{}},
	interfaces.SizeL:  {InstanceType: "firewall-rule", Specs: map[string]any{}},
	interfaces.SizeXL: {InstanceType: "firewall-rule", Specs: map[string]any{}},
}

var iamRoleSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "custom-role", Specs: map[string]any{}},
	interfaces.SizeS:  {InstanceType: "custom-role", Specs: map[string]any{}},
	interfaces.SizeM:  {InstanceType: "custom-role", Specs: map[string]any{}},
	interfaces.SizeL:  {InstanceType: "custom-role", Specs: map[string]any{}},
	interfaces.SizeXL: {InstanceType: "custom-role", Specs: map[string]any{}},
}

var storageSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "STANDARD", Specs: map[string]any{"storage_class": "STANDARD"}},
	interfaces.SizeS:  {InstanceType: "STANDARD", Specs: map[string]any{"storage_class": "STANDARD"}},
	interfaces.SizeM:  {InstanceType: "NEARLINE", Specs: map[string]any{"storage_class": "NEARLINE"}},
	interfaces.SizeL:  {InstanceType: "COLDLINE", Specs: map[string]any{"storage_class": "COLDLINE"}},
	interfaces.SizeXL: {InstanceType: "ARCHIVE", Specs: map[string]any{"storage_class": "ARCHIVE"}},
}

var certificateSizing = map[interfaces.Size]sizingEntry{
	interfaces.SizeXS: {InstanceType: "managed-ssl", Specs: map[string]any{"type": "MANAGED"}},
	interfaces.SizeS:  {InstanceType: "managed-ssl", Specs: map[string]any{"type": "MANAGED"}},
	interfaces.SizeM:  {InstanceType: "managed-ssl", Specs: map[string]any{"type": "MANAGED"}},
	interfaces.SizeL:  {InstanceType: "managed-ssl", Specs: map[string]any{"type": "MANAGED"}},
	interfaces.SizeXL: {InstanceType: "managed-ssl", Specs: map[string]any{"type": "MANAGED"}},
}

var sizingTables = map[string]map[interfaces.Size]sizingEntry{
	"infra.container_service": containerServiceSizing,
	"infra.k8s_cluster":       k8sClusterSizing,
	"infra.database":          databaseSizing,
	"infra.cache":             cacheSizing,
	"infra.vpc":               vpcSizing,
	"infra.load_balancer":     loadBalancerSizing,
	"infra.dns":               dnsSizing,
	"infra.registry":          registrySizing,
	"infra.api_gateway":       apiGatewaySizing,
	"infra.firewall":          firewallSizing,
	"infra.iam_role":          iamRoleSizing,
	"infra.storage":           storageSizing,
	"infra.certificate":       certificateSizing,
}

func resolveSizing(resourceType string, size interfaces.Size, hints *interfaces.ResourceHints) (*interfaces.ProviderSizing, error) {
	table, ok := sizingTables[resourceType]
	if !ok {
		return nil, fmt.Errorf("no sizing table for resource type %q", resourceType)
	}
	entry, ok := table[size]
	if !ok {
		return nil, fmt.Errorf("unknown size %q for resource type %q", size, resourceType)
	}

	specs := make(map[string]any, len(entry.Specs))
	for k, v := range entry.Specs {
		specs[k] = v
	}

	// Apply hints overrides.
	if hints != nil {
		if hints.CPU != "" {
			specs["cpu"] = hints.CPU
		}
		if hints.Memory != "" {
			specs["memory"] = hints.Memory
		}
		if hints.Storage != "" {
			specs["disk_size_gb"] = hints.Storage
		}
	}

	return &interfaces.ProviderSizing{
		InstanceType: entry.InstanceType,
		Specs:        specs,
	}, nil
}
