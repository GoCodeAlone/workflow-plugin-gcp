package provider

import (
	"context"
	"errors"
	"testing"

	assetpb "cloud.google.com/go/asset/apiv1/assetpb"
	"github.com/GoCodeAlone/workflow/interfaces"
)

type fakeOwnershipAssetClient struct {
	calls   []ownershipSearchCall
	results []*assetpb.ResourceSearchResult
}

type ownershipSearchCall struct {
	scope      string
	query      string
	assetTypes []string
}

func (f *fakeOwnershipAssetClient) SearchAllResources(ctx context.Context, scope, query string, assetTypes []string) ([]*assetpb.ResourceSearchResult, error) {
	f.calls = append(f.calls, ownershipSearchCall{
		scope:      scope,
		query:      query,
		assetTypes: append([]string(nil), assetTypes...),
	})
	return f.results, nil
}

type fakeOwnershipDriver struct {
	updateRefs  []interfaces.ResourceRef
	updateSpecs []interfaces.ResourceSpec
}

func (d *fakeOwnershipDriver) Create(context.Context, interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	return nil, errors.New("not used")
}

func (d *fakeOwnershipDriver) Read(_ context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	return &interfaces.ResourceOutput{Name: ref.Name, Type: ref.Type, ProviderID: ref.ProviderID, Status: "active"}, nil
}

func (d *fakeOwnershipDriver) Update(_ context.Context, ref interfaces.ResourceRef, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	d.updateRefs = append(d.updateRefs, ref)
	d.updateSpecs = append(d.updateSpecs, spec)
	return &interfaces.ResourceOutput{Name: ref.Name, Type: ref.Type, ProviderID: ref.ProviderID, Status: "active"}, nil
}

func (d *fakeOwnershipDriver) Delete(context.Context, interfaces.ResourceRef) error { return nil }

func (d *fakeOwnershipDriver) Diff(context.Context, interfaces.ResourceSpec, *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
	return &interfaces.DiffResult{}, nil
}

func (d *fakeOwnershipDriver) HealthCheck(context.Context, interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	return &interfaces.HealthResult{Healthy: true}, nil
}

func (d *fakeOwnershipDriver) Scale(context.Context, interfaces.ResourceRef, int) (*interfaces.ResourceOutput, error) {
	return nil, errors.New("not used")
}

func (d *fakeOwnershipDriver) SensitiveKeys() []string { return nil }

func TestOwnershipProviderCompileGuard(t *testing.T) {
	var _ interfaces.OwnershipProvider = (*GCPProvider)(nil)
}

func TestSetOwnerUpdatesWorkflowOwnerLabelThroughDriver(t *testing.T) {
	driver := &fakeOwnershipDriver{}
	p := initializedOwnershipProvider(&fakeOwnershipAssetClient{})
	p.SetDriver("infra.container_service", driver)
	ref := interfaces.ResourceRef{Name: "api", Type: "infra.container_service", ProviderID: "projects/proj/locations/us-central1/services/api"}

	if err := p.SetOwner(context.Background(), ref, "workflow"); err != nil {
		t.Fatalf("SetOwner: %v", err)
	}

	if len(driver.updateRefs) != 1 || driver.updateRefs[0] != ref {
		t.Fatalf("update refs = %#v, want %#v", driver.updateRefs, ref)
	}
	labels, ok := driver.updateSpecs[0].Config["labels"].(map[string]string)
	if !ok {
		t.Fatalf("labels config = %#v, want map[string]string", driver.updateSpecs[0].Config["labels"])
	}
	if labels[ownershipLabelKey] != "workflow" {
		t.Fatalf("labels[%q] = %q, want workflow", ownershipLabelKey, labels[ownershipLabelKey])
	}
}

func TestGetOwnerReadsWorkflowOwnerLabelFromCloudAsset(t *testing.T) {
	ref := interfaces.ResourceRef{Name: "api", Type: "infra.container_service", ProviderID: "//run.googleapis.com/projects/proj/locations/us-central1/services/api"}
	assetClient := &fakeOwnershipAssetClient{
		results: []*assetpb.ResourceSearchResult{
			{
				Name:      ref.ProviderID,
				AssetType: "run.googleapis.com/Service",
				Labels:    map[string]string{ownershipLabelKey: "workflow"},
			},
		},
	}
	p := initializedOwnershipProvider(assetClient)

	owner, err := p.GetOwner(context.Background(), ref)
	if err != nil {
		t.Fatalf("GetOwner: %v", err)
	}
	if owner.Owner != "workflow" || owner.Source != ownershipLabelSource {
		t.Fatalf("owner = %#v, want workflow source %q", owner, ownershipLabelSource)
	}
	if len(assetClient.calls) != 1 {
		t.Fatalf("search calls = %d, want 1", len(assetClient.calls))
	}
	if assetClient.calls[0].scope != "projects/proj" {
		t.Fatalf("scope = %q, want projects/proj", assetClient.calls[0].scope)
	}
	if assetClient.calls[0].query != `name:"//run.googleapis.com/projects/proj/locations/us-central1/services/api"` {
		t.Fatalf("query = %q", assetClient.calls[0].query)
	}
}

func TestGetOwnerExpandsBareProviderIDToCloudAssetName(t *testing.T) {
	assetClient := &fakeOwnershipAssetClient{
		results: []*assetpb.ResourceSearchResult{
			{
				Name:      "//run.googleapis.com/projects/proj/locations/us-central1/services/api",
				AssetType: "run.googleapis.com/Service",
				Labels:    map[string]string{ownershipLabelKey: "workflow"},
			},
		},
	}
	p := initializedOwnershipProvider(assetClient)

	owner, err := p.GetOwner(context.Background(), interfaces.ResourceRef{Name: "api", Type: "infra.container_service", ProviderID: "api"})
	if err != nil {
		t.Fatalf("GetOwner: %v", err)
	}
	if owner.Owner != "workflow" {
		t.Fatalf("Owner = %q, want workflow", owner.Owner)
	}
	if assetClient.calls[0].query != `name:"//run.googleapis.com/projects/proj/locations/us-central1/services/api"` {
		t.Fatalf("query = %q", assetClient.calls[0].query)
	}
}

func TestListOwnersFiltersByOwnerAndResourceType(t *testing.T) {
	assetClient := &fakeOwnershipAssetClient{
		results: []*assetpb.ResourceSearchResult{
			{
				Name:        "//run.googleapis.com/projects/proj/locations/us-central1/services/api",
				AssetType:   "run.googleapis.com/Service",
				DisplayName: "api",
				Labels:      map[string]string{ownershipLabelKey: "workflow"},
			},
			{
				Name:        "//sqladmin.googleapis.com/projects/proj/instances/db",
				AssetType:   "sqladmin.googleapis.com/Instance",
				DisplayName: "db",
				Labels:      map[string]string{ownershipLabelKey: "workflow"},
			},
		},
	}
	p := initializedOwnershipProvider(assetClient)

	owners, err := p.ListOwners(context.Background(), interfaces.OwnerFilter{Owner: "workflow", ResourceType: "infra.container_service"})
	if err != nil {
		t.Fatalf("ListOwners: %v", err)
	}
	if len(assetClient.calls) != 1 {
		t.Fatalf("search calls = %d, want 1", len(assetClient.calls))
	}
	if assetClient.calls[0].query != `labels.workflow-owner:"workflow"` {
		t.Fatalf("query = %q", assetClient.calls[0].query)
	}
	if len(assetClient.calls[0].assetTypes) != 1 || assetClient.calls[0].assetTypes[0] != "run.googleapis.com/Service" {
		t.Fatalf("assetTypes = %v", assetClient.calls[0].assetTypes)
	}
	if len(owners) != 1 {
		t.Fatalf("owners len = %d, want 1: %#v", len(owners), owners)
	}
	got := owners[0]
	if got.Owner != "workflow" || got.Source != ownershipLabelSource {
		t.Fatalf("owner = %#v, want workflow source %q", got, ownershipLabelSource)
	}
	if got.Ref.Name != "api" || got.Ref.Type != "infra.container_service" || got.Ref.ProviderID != "//run.googleapis.com/projects/proj/locations/us-central1/services/api" {
		t.Fatalf("ref = %#v", got.Ref)
	}
}

func initializedOwnershipProvider(assetClient ownershipAssetClient) *GCPProvider {
	return &GCPProvider{
		projectID:       "proj",
		region:          "us-central1",
		zone:            "us-central1-a",
		drivers:         make(map[string]interfaces.ResourceDriver),
		ownershipAssets: assetClient,
	}
}
