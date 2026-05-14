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

// TestGKEDriver_Lifecycle exercises the full GKE lifecycle through the
// ResourceDriver contract (create → read → delete), the cross-process path the
// workflow-core grpcKubernetesBackend adapter drives (ADR 0037). It also pins
// the Read output-key set the host adapter projects onto KubernetesClusterState.
func TestGKEDriver_Lifecycle(t *testing.T) {
	mock := &mockGKEClient{getResult: map[string]any{
		"name":     "my-cluster",
		"status":   "RUNNING",
		"endpoint": "https://1.2.3.4",
		"version":  "1.29.1",
		"nodeGroups": []map[string]any{
			{"name": "default-pool", "instanceType": "e2-medium", "min": 1, "max": 3, "current": 2},
		},
	}}
	d := &GKEDriver{Client: mock, ProjectID: "test-project", Location: "us-central1-a"}

	spec := interfaces.ResourceSpec{
		Name: "my-cluster", Type: "infra.k8s_cluster",
		Config: map[string]any{"name": "my-cluster", "machine_type": "e2-medium"},
	}
	created, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	ref := interfaces.ResourceRef{Name: "my-cluster", Type: "infra.k8s_cluster", ProviderID: created.ProviderID}
	read, err := d.Read(context.Background(), ref)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	// Pinned output-key contract (ADR 0037): the workflow-core host adapter
	// reads exactly these keys to reconstruct KubernetesClusterState.
	for _, key := range []string{"status", "endpoint", "version", "nodeGroups"} {
		if _, ok := read.Outputs[key]; !ok {
			t.Errorf("Read output missing pinned key %q (host adapter depends on it)", key)
		}
	}

	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

// TestGKEDriver_Create_IdempotentOnAlreadyExists verifies Create resolves an
// ALREADY_EXISTS response to success — preserving the in-core gkeBackend
// behavior the cross-process path must keep (ADR 0037 Consequences).
func TestGKEDriver_Create_IdempotentOnAlreadyExists(t *testing.T) {
	d := &GKEDriver{
		Client:    &mockGKEClient{createErr: fmt.Errorf("rpc error: code = AlreadyExists desc = Already Exists")},
		ProjectID: "test-project",
		Location:  "us-central1-a",
	}
	spec := interfaces.ResourceSpec{
		Name: "dup-cluster", Type: "infra.k8s_cluster",
		Config: map[string]any{"name": "dup-cluster"},
	}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("Create on ALREADY_EXISTS must resolve to success, got: %v", err)
	}
	if out == nil || out.ProviderID == "" {
		t.Fatalf("Create on ALREADY_EXISTS must return a populated output, got: %+v", out)
	}
}

// TestGKEDriver_Delete_IdempotentOnNotFound verifies Delete resolves a NOT_FOUND
// response to success — preserving the in-core gkeBackend behavior (ADR 0037).
func TestGKEDriver_Delete_IdempotentOnNotFound(t *testing.T) {
	d := &GKEDriver{
		Client:    &mockGKEClient{deleteErr: fmt.Errorf("rpc error: code = NotFound desc = NOT_FOUND")},
		ProjectID: "test-project",
		Location:  "us-central1-a",
	}
	ref := interfaces.ResourceRef{Name: "gone", Type: "infra.k8s_cluster", ProviderID: "gone"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("Delete on NOT_FOUND must resolve to success, got: %v", err)
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
