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
