package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockArtifactRegistryClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockArtifactRegistryClient) CreateRepository(_ context.Context, _, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "repo-123", nil
}

func (m *mockArtifactRegistryClient) GetRepository(_ context.Context, _, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"repository_id": "repo-123"}, nil
}

func (m *mockArtifactRegistryClient) UpdateRepository(_ context.Context, _, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockArtifactRegistryClient) DeleteRepository(_ context.Context, _, _, _ string) error {
	return m.deleteErr
}

func TestArtifactRegistryDriver_Create_Success(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{}, ProjectID: "p", Location: "us-central1"}
	spec := interfaces.ResourceSpec{Name: "myrepo", Type: "infra.registry", Config: map[string]any{"format": "DOCKER"}}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Outputs["registry_url"] == nil {
		t.Error("expected registry_url")
	}
}

func TestArtifactRegistryDriver_Create_Error(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{createErr: fmt.Errorf("fail")}, ProjectID: "p", Location: "us"}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.registry", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestArtifactRegistryDriver_Update_Success(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{}, ProjectID: "p", Location: "us"}
	ref := interfaces.ResourceRef{Name: "repo", Type: "infra.registry", ProviderID: "repo-123"}
	spec := interfaces.ResourceSpec{Name: "repo", Config: map[string]any{"description": "updated"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestArtifactRegistryDriver_Update_Error(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p", Location: "us"}
	ref := interfaces.ResourceRef{Name: "repo", Type: "infra.registry", ProviderID: "repo-123"}
	spec := interfaces.ResourceSpec{Name: "repo", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestArtifactRegistryDriver_Delete_Success(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{}, ProjectID: "p", Location: "us"}
	ref := interfaces.ResourceRef{Name: "repo", Type: "infra.registry", ProviderID: "repo-123"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestArtifactRegistryDriver_Delete_Error(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p", Location: "us"}
	ref := interfaces.ResourceRef{Name: "repo", Type: "infra.registry", ProviderID: "repo-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestArtifactRegistryDriver_Diff_NeedsReplace(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{}, ProjectID: "p", Location: "us"}
	spec := interfaces.ResourceSpec{Name: "repo", Config: map[string]any{"format": "MAVEN"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"format": "DOCKER"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsReplace {
		t.Error("expected needs replace for format change")
	}
}

func TestArtifactRegistryDriver_Diff_NoChanges(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{}, ProjectID: "p", Location: "us"}
	spec := interfaces.ResourceSpec{Name: "repo", Config: map[string]any{"format": "DOCKER"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"format": "DOCKER"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestArtifactRegistryDriver_HealthCheck_Healthy(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{}, ProjectID: "p", Location: "us"}
	ref := interfaces.ResourceRef{Name: "repo", Type: "infra.registry", ProviderID: "repo-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}

func TestArtifactRegistryDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &ArtifactRegistryDriver{Client: &mockArtifactRegistryClient{getErr: fmt.Errorf("repo not found")}, ProjectID: "p", Location: "us"}
	ref := interfaces.ResourceRef{Name: "repo", Type: "infra.registry", ProviderID: "repo-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
