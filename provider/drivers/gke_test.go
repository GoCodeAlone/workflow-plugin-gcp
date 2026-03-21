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

func TestGKEDriver_Update_Success(t *testing.T) {
	d := &GKEDriver{Client: &mockGKEClient{}, ProjectID: "p", Location: "z"}
	ref := interfaces.ResourceRef{Name: "cluster", Type: "infra.k8s_cluster", ProviderID: "cluster-123"}
	spec := interfaces.ResourceSpec{Name: "cluster", Config: map[string]any{"node_count": 5}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestGKEDriver_Update_Error(t *testing.T) {
	d := &GKEDriver{Client: &mockGKEClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p", Location: "z"}
	ref := interfaces.ResourceRef{Name: "cluster", Type: "infra.k8s_cluster", ProviderID: "cluster-123"}
	spec := interfaces.ResourceSpec{Name: "cluster", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGKEDriver_Delete_Success(t *testing.T) {
	d := &GKEDriver{Client: &mockGKEClient{}, ProjectID: "p", Location: "z"}
	ref := interfaces.ResourceRef{Name: "cluster", Type: "infra.k8s_cluster", ProviderID: "cluster-123"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGKEDriver_Delete_Error(t *testing.T) {
	d := &GKEDriver{Client: &mockGKEClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p", Location: "z"}
	ref := interfaces.ResourceRef{Name: "cluster", Type: "infra.k8s_cluster", ProviderID: "cluster-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestGKEDriver_Diff_NoChanges(t *testing.T) {
	d := &GKEDriver{Client: &mockGKEClient{}, ProjectID: "p", Location: "z"}
	spec := interfaces.ResourceSpec{Name: "c", Config: map[string]any{"machine_type": "n2-standard-2"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"machine_type": "n2-standard-2"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestGKEDriver_Diff_HasChanges(t *testing.T) {
	d := &GKEDriver{Client: &mockGKEClient{}, ProjectID: "p", Location: "z"}
	spec := interfaces.ResourceSpec{Name: "c", Config: map[string]any{"machine_type": "n2-standard-4"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"machine_type": "n2-standard-2"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsUpdate {
		t.Error("expected update needed")
	}
}

func TestGKEDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &GKEDriver{Client: &mockGKEClient{getErr: fmt.Errorf("cluster not found")}, ProjectID: "p", Location: "z"}
	ref := interfaces.ResourceRef{Name: "cluster", Type: "infra.k8s_cluster", ProviderID: "cluster-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
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
