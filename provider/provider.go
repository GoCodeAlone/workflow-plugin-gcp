package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/GoCodeAlone/workflow/interfaces"
	"github.com/GoCodeAlone/workflow-plugin-gcp/provider/drivers"
)

const (
	providerName    = "gcp"
	providerVersion = "0.1.0"
)

// GCPProvider implements interfaces.IaCProvider for Google Cloud Platform.
type GCPProvider struct {
	projectID string
	region    string
	zone      string
	drivers   map[string]interfaces.ResourceDriver
}

// New creates a new uninitialized GCPProvider.
func New() *GCPProvider {
	return &GCPProvider{
		drivers: make(map[string]interfaces.ResourceDriver),
	}
}

func (p *GCPProvider) Name() string    { return providerName }
func (p *GCPProvider) Version() string { return providerVersion }

func (p *GCPProvider) Initialize(_ context.Context, config map[string]any) error {
	pid, ok := config["project_id"].(string)
	if !ok || pid == "" {
		return fmt.Errorf("gcp: project_id is required")
	}
	p.projectID = pid

	p.region = "us-central1"
	if r, ok := config["region"].(string); ok && r != "" {
		p.region = r
	}

	p.zone = "us-central1-a"
	if z, ok := config["zone"].(string); ok && z != "" {
		p.zone = z
	}

	// Register all drivers with nil clients (real clients would be created here
	// via ADC or credentials_file). The nil-client drivers will fail at call time,
	// which is the correct behavior when no real GCP credentials are available.
	// Tests inject mock clients directly.
	p.registerDrivers()
	return nil
}

func (p *GCPProvider) registerDrivers() {
	p.drivers["infra.container_service"] = &drivers.CloudRunDriver{ProjectID: p.projectID, Region: p.region}
	p.drivers["infra.k8s_cluster"] = &drivers.GKEDriver{ProjectID: p.projectID, Location: p.zone}
	p.drivers["infra.database"] = &drivers.CloudSQLDriver{ProjectID: p.projectID, Region: p.region}
	p.drivers["infra.cache"] = &drivers.MemorystoreDriver{ProjectID: p.projectID, Region: p.region}
	p.drivers["infra.vpc"] = &drivers.VPCDriver{ProjectID: p.projectID, Region: p.region}
	p.drivers["infra.load_balancer"] = &drivers.LoadBalancerDriver{ProjectID: p.projectID, Region: p.region}
	p.drivers["infra.dns"] = &drivers.DNSDriver{ProjectID: p.projectID}
	p.drivers["infra.registry"] = &drivers.ArtifactRegistryDriver{ProjectID: p.projectID, Location: p.region}
	p.drivers["infra.api_gateway"] = &drivers.APIGatewayDriver{ProjectID: p.projectID, Region: p.region}
	p.drivers["infra.firewall"] = &drivers.FirewallDriver{ProjectID: p.projectID}
	p.drivers["infra.iam_role"] = &drivers.IAMDriver{ProjectID: p.projectID}
	p.drivers["infra.storage"] = &drivers.GCSDriver{ProjectID: p.projectID}
	p.drivers["infra.certificate"] = &drivers.SSLCertificateDriver{ProjectID: p.projectID}
}

// SetDriver allows injecting a driver (used in tests and when wiring real clients).
func (p *GCPProvider) SetDriver(resourceType string, d interfaces.ResourceDriver) {
	p.drivers[resourceType] = d
}

func (p *GCPProvider) Capabilities() []interfaces.IaCCapabilityDeclaration {
	allOps := []string{"create", "read", "update", "delete"}
	scalableOps := []string{"create", "read", "update", "delete", "scale"}

	return []interfaces.IaCCapabilityDeclaration{
		{ResourceType: "infra.container_service", Tier: 3, Operations: scalableOps},
		{ResourceType: "infra.k8s_cluster", Tier: 1, Operations: scalableOps},
		{ResourceType: "infra.database", Tier: 2, Operations: allOps},
		{ResourceType: "infra.cache", Tier: 2, Operations: allOps},
		{ResourceType: "infra.vpc", Tier: 1, Operations: allOps},
		{ResourceType: "infra.load_balancer", Tier: 1, Operations: allOps},
		{ResourceType: "infra.dns", Tier: 1, Operations: allOps},
		{ResourceType: "infra.registry", Tier: 2, Operations: allOps},
		{ResourceType: "infra.api_gateway", Tier: 2, Operations: allOps},
		{ResourceType: "infra.firewall", Tier: 1, Operations: allOps},
		{ResourceType: "infra.iam_role", Tier: 1, Operations: allOps},
		{ResourceType: "infra.storage", Tier: 2, Operations: allOps},
		{ResourceType: "infra.certificate", Tier: 2, Operations: allOps},
	}
}

func (p *GCPProvider) Plan(ctx context.Context, desired []interfaces.ResourceSpec, current []interfaces.ResourceState) (*interfaces.IaCPlan, error) {
	currentMap := make(map[string]*interfaces.ResourceState, len(current))
	for i := range current {
		currentMap[current[i].Name] = &current[i]
	}

	plan := &interfaces.IaCPlan{
		ID:        fmt.Sprintf("plan-%d", time.Now().UnixNano()),
		CreatedAt: time.Now(),
	}

	for _, spec := range desired {
		cur, exists := currentMap[spec.Name]
		if !exists {
			plan.Actions = append(plan.Actions, interfaces.PlanAction{
				Action:   "create",
				Resource: spec,
			})
			continue
		}

		// Check for drift by comparing config.
		drv, err := p.ResourceDriver(spec.Type)
		if err != nil {
			return nil, fmt.Errorf("plan: %w", err)
		}

		curOutput := &interfaces.ResourceOutput{
			Name:       cur.Name,
			Type:       cur.Type,
			ProviderID: cur.ProviderID,
			Outputs:    cur.Outputs,
			Status:     "running",
		}

		diff, err := drv.Diff(ctx, spec, curOutput)
		if err != nil {
			return nil, fmt.Errorf("plan diff %s: %w", spec.Name, err)
		}

		if diff.NeedsReplace {
			plan.Actions = append(plan.Actions, interfaces.PlanAction{
				Action:   "replace",
				Resource: spec,
				Current:  cur,
				Changes:  diff.Changes,
			})
		} else if diff.NeedsUpdate {
			plan.Actions = append(plan.Actions, interfaces.PlanAction{
				Action:   "update",
				Resource: spec,
				Current:  cur,
				Changes:  diff.Changes,
			})
		}
	}

	return plan, nil
}

func (p *GCPProvider) Apply(ctx context.Context, plan *interfaces.IaCPlan) (*interfaces.ApplyResult, error) {
	result := &interfaces.ApplyResult{PlanID: plan.ID}

	for _, action := range plan.Actions {
		drv, err := p.ResourceDriver(action.Resource.Type)
		if err != nil {
			result.Errors = append(result.Errors, interfaces.ActionError{
				Resource: action.Resource.Name, Action: action.Action, Error: err.Error(),
			})
			continue
		}

		var out *interfaces.ResourceOutput

		switch action.Action {
		case "create":
			out, err = drv.Create(ctx, action.Resource)
		case "update":
			ref := interfaces.ResourceRef{
				Name: action.Resource.Name, Type: action.Resource.Type,
			}
			if action.Current != nil {
				ref.ProviderID = action.Current.ProviderID
			}
			out, err = drv.Update(ctx, ref, action.Resource)
		case "replace":
			if action.Current != nil {
				ref := interfaces.ResourceRef{
					Name: action.Current.Name, Type: action.Current.Type, ProviderID: action.Current.ProviderID,
				}
				_ = drv.Delete(ctx, ref)
			}
			out, err = drv.Create(ctx, action.Resource)
		case "delete":
			ref := interfaces.ResourceRef{
				Name: action.Resource.Name, Type: action.Resource.Type,
			}
			if action.Current != nil {
				ref.ProviderID = action.Current.ProviderID
			}
			err = drv.Delete(ctx, ref)
		}

		if err != nil {
			result.Errors = append(result.Errors, interfaces.ActionError{
				Resource: action.Resource.Name, Action: action.Action, Error: err.Error(),
			})
			continue
		}
		if out != nil {
			result.Resources = append(result.Resources, *out)
		}
	}

	return result, nil
}

func (p *GCPProvider) Destroy(ctx context.Context, resources []interfaces.ResourceRef) (*interfaces.DestroyResult, error) {
	result := &interfaces.DestroyResult{}
	for _, ref := range resources {
		drv, err := p.ResourceDriver(ref.Type)
		if err != nil {
			result.Errors = append(result.Errors, interfaces.ActionError{
				Resource: ref.Name, Action: "delete", Error: err.Error(),
			})
			continue
		}
		if err := drv.Delete(ctx, ref); err != nil {
			result.Errors = append(result.Errors, interfaces.ActionError{
				Resource: ref.Name, Action: "delete", Error: err.Error(),
			})
			continue
		}
		result.Destroyed = append(result.Destroyed, ref.Name)
	}
	return result, nil
}

func (p *GCPProvider) Status(ctx context.Context, resources []interfaces.ResourceRef) ([]interfaces.ResourceStatus, error) {
	var statuses []interfaces.ResourceStatus
	for _, ref := range resources {
		drv, err := p.ResourceDriver(ref.Type)
		if err != nil {
			statuses = append(statuses, interfaces.ResourceStatus{
				Name: ref.Name, Type: ref.Type, ProviderID: ref.ProviderID, Status: "unknown",
			})
			continue
		}
		out, err := drv.Read(ctx, ref)
		if err != nil {
			statuses = append(statuses, interfaces.ResourceStatus{
				Name: ref.Name, Type: ref.Type, ProviderID: ref.ProviderID, Status: "unknown",
			})
			continue
		}
		statuses = append(statuses, interfaces.ResourceStatus{
			Name: out.Name, Type: out.Type, ProviderID: out.ProviderID,
			Status: out.Status, Outputs: out.Outputs,
		})
	}
	return statuses, nil
}

func (p *GCPProvider) DetectDrift(ctx context.Context, resources []interfaces.ResourceRef) ([]interfaces.DriftResult, error) {
	var results []interfaces.DriftResult
	for _, ref := range resources {
		drv, err := p.ResourceDriver(ref.Type)
		if err != nil {
			results = append(results, interfaces.DriftResult{
				Name: ref.Name, Type: ref.Type, Drifted: false,
			})
			continue
		}
		out, err := drv.Read(ctx, ref)
		if err != nil {
			results = append(results, interfaces.DriftResult{
				Name: ref.Name, Type: ref.Type, Drifted: false,
			})
			continue
		}
		results = append(results, interfaces.DriftResult{
			Name:   ref.Name,
			Type:   ref.Type,
			Actual: out.Outputs,
		})
	}
	return results, nil
}

func (p *GCPProvider) Import(ctx context.Context, cloudID string, resourceType string) (*interfaces.ResourceState, error) {
	drv, err := p.ResourceDriver(resourceType)
	if err != nil {
		return nil, fmt.Errorf("import: %w", err)
	}
	ref := interfaces.ResourceRef{
		Name: cloudID, Type: resourceType, ProviderID: cloudID,
	}
	out, err := drv.Read(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("import read: %w", err)
	}
	return &interfaces.ResourceState{
		ID:         cloudID,
		Name:       out.Name,
		Type:       out.Type,
		Provider:   providerName,
		ProviderID: out.ProviderID,
		Outputs:    out.Outputs,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (p *GCPProvider) ResolveSizing(resourceType string, size interfaces.Size, hints *interfaces.ResourceHints) (*interfaces.ProviderSizing, error) {
	return resolveSizing(resourceType, size, hints)
}

func (p *GCPProvider) ResourceDriver(resourceType string) (interfaces.ResourceDriver, error) {
	drv, ok := p.drivers[resourceType]
	if !ok {
		return nil, fmt.Errorf("gcp: unsupported resource type %q", resourceType)
	}
	return drv, nil
}

func (p *GCPProvider) Close() error { return nil }
