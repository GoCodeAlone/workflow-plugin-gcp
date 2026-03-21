package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockGKEClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockGKEClient) CreateCluster(_ context.Context, _, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "cluster-123", nil
}

func (m *mockGKEClient) GetCluster(_ context.Context, _, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"status": "RUNNING"}, nil
}

func (m *mockGKEClient) UpdateCluster(_ context.Context, _, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockGKEClient) DeleteCluster(_ context.Context, _, _, _ string) error {
	return m.deleteErr
}

func TestGKEDriver_Create_Success(t *testing.T) {
	d := &GKEDriver{
		Client:    &mockGKEClient{},
		ProjectID: "test-project",
		Location:  "us-central1-a",
	}
	spec := interfaces.ResourceSpec{
		Name: "my-cluster", Type: "infra.k8s_cluster",
		Config: map[string]any{"machine_type": "n2-standard-2"},
	}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "cluster-123" {
		t.Errorf("expected cluster-123, got %s", out.ProviderID)
	}
}

func TestGKEDriver_Create_Error(t *testing.T) {
	d := &GKEDriver{
		Client:    &mockGKEClient{createErr: fmt.Errorf("insufficient quota")},
		ProjectID: "test-project",
		Location:  "us-central1-a",
	}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.k8s_cluster", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGKEDriver_Scale(t *testing.T) {
	d := &GKEDriver{
		Client:    &mockGKEClient{},
		ProjectID: "test-project",
		Location:  "us-central1-a",
	}
	ref := interfaces.ResourceRef{Name: "cluster", Type: "infra.k8s_cluster", ProviderID: "cluster-123"}
	out, err := d.Scale(context.Background(), ref, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}
