package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	asset "cloud.google.com/go/asset/apiv1"
	assetpb "cloud.google.com/go/asset/apiv1/assetpb"
	"github.com/GoCodeAlone/workflow/interfaces"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	ownershipLabelKey    = "workflow-owner"
	ownershipLabelSource = "label:workflow-owner"
)

var ErrOwnershipUnsupportedResource = errors.New("gcp ownership labels are not supported for this resource type")

type ownershipAssetClient interface {
	SearchAllResources(ctx context.Context, scope, query string, assetTypes []string) ([]*assetpb.ResourceSearchResult, error)
}

type ownershipAssetClientCloser interface {
	ownershipAssetClient
	Close() error
}

type gcpOwnershipAssetClient struct {
	inner *asset.Client
}

func (c *gcpOwnershipAssetClient) SearchAllResources(ctx context.Context, scope, query string, assetTypes []string) ([]*assetpb.ResourceSearchResult, error) {
	it := c.inner.SearchAllResources(ctx, &assetpb.SearchAllResourcesRequest{
		Scope:      scope,
		Query:      query,
		AssetTypes: assetTypes,
		ReadMask: &fieldmaskpb.FieldMask{
			Paths: []string{"name", "asset_type", "display_name", "labels"},
		},
	})
	var out []*assetpb.ResourceSearchResult
	for {
		result, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		out = append(out, result)
	}
	return out, nil
}

func (c *gcpOwnershipAssetClient) Close() error {
	return c.inner.Close()
}

func (p *GCPProvider) initializeOwnershipAssets(ctx context.Context, opts []option.ClientOption) error {
	client, err := asset.NewClient(ctx, opts...)
	if err != nil {
		p.ownershipAssets = nil
		return nil
	}
	p.ownershipAssets = &gcpOwnershipAssetClient{inner: client}
	return nil
}

func (p *GCPProvider) closeOwnershipAssets() error {
	if closer, ok := p.ownershipAssets.(ownershipAssetClientCloser); ok {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	p.ownershipAssets = nil
	return nil
}

func (p *GCPProvider) GetOwner(ctx context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOwner, error) {
	client, err := p.ownershipAssetClient()
	if err != nil {
		return nil, err
	}
	assetTypes := assetTypesForWorkflowType(ref.Type)
	if len(assetTypes) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrOwnershipUnsupportedResource, ref.Type)
	}
	assetName := p.ownershipAssetName(ref)
	results, err := client.SearchAllResources(ctx, p.ownershipScope(), fmt.Sprintf("name:%q", assetName), assetTypes)
	if err != nil {
		return nil, fmt.Errorf("gcp: get ownership labels for %q: %w", ref.Name, err)
	}
	for _, result := range results {
		if result.GetName() == assetName {
			return &interfaces.ResourceOwner{
				Ref:    ref,
				Owner:  result.GetLabels()[ownershipLabelKey],
				Source: ownershipLabelSource,
			}, nil
		}
	}
	return &interfaces.ResourceOwner{Ref: ref, Source: ownershipLabelSource}, nil
}

func (p *GCPProvider) SetOwner(ctx context.Context, ref interfaces.ResourceRef, owner string) error {
	if strings.TrimSpace(owner) == "" {
		return fmt.Errorf("gcp: owner must be non-empty")
	}
	if len(assetTypesForWorkflowType(ref.Type)) == 0 {
		return fmt.Errorf("%w: %s", ErrOwnershipUnsupportedResource, ref.Type)
	}
	driver, err := p.ResourceDriver(ref.Type)
	if err != nil {
		return err
	}
	driverRef, err := ownershipDriverRef(ref)
	if err != nil {
		return err
	}
	_, err = driver.Update(ctx, driverRef, interfaces.ResourceSpec{
		Name: ref.Name,
		Type: ref.Type,
		Config: map[string]any{
			"labels": map[string]string{ownershipLabelKey: owner},
		},
	})
	if err != nil {
		return fmt.Errorf("gcp: label %s/%s with owner %q: %w", ref.Type, ref.Name, owner, err)
	}
	return nil
}

func ownershipDriverRef(ref interfaces.ResourceRef) (interfaces.ResourceRef, error) {
	id := strings.TrimSpace(ref.ProviderID)
	if id == "" {
		id = strings.TrimSpace(ref.Name)
	}
	id = nameFromGCPAssetName(id)
	if id == "" {
		return interfaces.ResourceRef{}, fmt.Errorf("gcp: resource %s/%s has no provider ID or name", ref.Type, ref.Name)
	}
	ref.ProviderID = id
	return ref, nil
}

func (p *GCPProvider) ListOwners(ctx context.Context, filter interfaces.OwnerFilter) ([]interfaces.ResourceOwner, error) {
	client, err := p.ownershipAssetClient()
	if err != nil {
		return nil, err
	}
	assetTypes := assetTypesForWorkflowType(filter.ResourceType)
	if filter.ResourceType != "" && len(assetTypes) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrOwnershipUnsupportedResource, filter.ResourceType)
	}
	query := fmt.Sprintf("labels.%s:*", ownershipLabelKey)
	if filter.Owner != "" {
		query = fmt.Sprintf("labels.%s:%q", ownershipLabelKey, filter.Owner)
	}
	results, err := client.SearchAllResources(ctx, p.ownershipScope(), query, assetTypes)
	if err != nil {
		return nil, fmt.Errorf("gcp: list ownership labels: %w", err)
	}

	var owners []interfaces.ResourceOwner
	for _, result := range results {
		owner := result.GetLabels()[ownershipLabelKey]
		if owner == "" {
			continue
		}
		ref := refFromGCPAsset(result)
		if ref.ProviderID == "" {
			continue
		}
		if filter.ResourceType != "" && ref.Type != filter.ResourceType {
			continue
		}
		owners = append(owners, interfaces.ResourceOwner{Ref: ref, Owner: owner, Source: ownershipLabelSource})
	}
	return owners, nil
}

func (p *GCPProvider) ownershipAssetClient() (ownershipAssetClient, error) {
	if p.projectID == "" {
		return nil, fmt.Errorf("gcp: provider not initialized")
	}
	if p.ownershipAssets == nil {
		return nil, fmt.Errorf("gcp: ownership asset client not initialized")
	}
	return p.ownershipAssets, nil
}

func (p *GCPProvider) ownershipScope() string {
	return "projects/" + p.projectID
}

func (p *GCPProvider) ownershipAssetName(ref interfaces.ResourceRef) string {
	if strings.HasPrefix(ref.ProviderID, "//") {
		return ref.ProviderID
	}
	id := ref.ProviderID
	if id == "" {
		id = ref.Name
	}
	if strings.HasPrefix(id, "projects/") {
		assetTypes := assetTypesForWorkflowType(ref.Type)
		service := assetTypes[0][:strings.Index(assetTypes[0], "/")]
		return "//" + service + "/" + id
	}
	switch ref.Type {
	case "infra.container_service":
		return fmt.Sprintf("//run.googleapis.com/projects/%s/locations/%s/services/%s", p.projectID, p.region, id)
	case "infra.database":
		return fmt.Sprintf("//sqladmin.googleapis.com/projects/%s/instances/%s", p.projectID, id)
	case "infra.cache":
		return fmt.Sprintf("//redis.googleapis.com/projects/%s/locations/%s/instances/%s", p.projectID, p.region, id)
	case "infra.registry":
		return fmt.Sprintf("//artifactregistry.googleapis.com/projects/%s/locations/%s/repositories/%s", p.projectID, p.region, id)
	case "infra.storage":
		return fmt.Sprintf("//storage.googleapis.com/projects/_/buckets/%s", id)
	default:
		return id
	}
}

func refFromGCPAsset(result *assetpb.ResourceSearchResult) interfaces.ResourceRef {
	resourceType := workflowTypeFromGCPAssetType(result.GetAssetType())
	if resourceType == "" || result.GetName() == "" {
		return interfaces.ResourceRef{}
	}
	name := result.GetDisplayName()
	if name == "" {
		name = nameFromGCPAssetName(result.GetName())
	}
	return interfaces.ResourceRef{
		Name:       name,
		Type:       resourceType,
		ProviderID: result.GetName(),
	}
}

func workflowTypeFromGCPAssetType(assetType string) string {
	switch assetType {
	case "run.googleapis.com/Service":
		return "infra.container_service"
	case "sqladmin.googleapis.com/Instance":
		return "infra.database"
	case "redis.googleapis.com/Instance":
		return "infra.cache"
	case "artifactregistry.googleapis.com/Repository":
		return "infra.registry"
	case "storage.googleapis.com/Bucket":
		return "infra.storage"
	default:
		return ""
	}
}

func assetTypesForWorkflowType(resourceType string) []string {
	if resourceType == "" {
		return []string{
			"run.googleapis.com/Service",
			"sqladmin.googleapis.com/Instance",
			"redis.googleapis.com/Instance",
			"artifactregistry.googleapis.com/Repository",
			"storage.googleapis.com/Bucket",
		}
	}
	for assetType, workflowType := range map[string]string{
		"run.googleapis.com/Service":                 "infra.container_service",
		"sqladmin.googleapis.com/Instance":           "infra.database",
		"redis.googleapis.com/Instance":              "infra.cache",
		"artifactregistry.googleapis.com/Repository": "infra.registry",
		"storage.googleapis.com/Bucket":              "infra.storage",
	} {
		if workflowType == resourceType {
			return []string{assetType}
		}
	}
	return nil
}

func nameFromGCPAssetName(name string) string {
	name = strings.TrimRight(name, "/")
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "/")
	return parts[len(parts)-1]
}

var _ interfaces.OwnershipProvider = (*GCPProvider)(nil)
