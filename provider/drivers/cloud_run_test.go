package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockCloudRunClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockCloudRunClient) CreateService(_ context.Context, _, _ string, cfg map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	name, _ := cfg["name"].(string)
	if name == "" {
		name = "svc-123"
	}
	return name, nil
}

func (m *mockCloudRunClient) GetService(_ context.Context, _, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"status": "running"}, nil
}

func (m *mockCloudRunClient) UpdateService(_ context.Context, _, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockCloudRunClient) DeleteService(_ context.Context, _, _, _ string) error {
	return m.deleteErr
}

func TestCloudRunDriver_Create_Success(t *testing.T) {
	d := &CloudRunDriver{
		Client:    &mockCloudRunClient{},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	spec := interfaces.ResourceSpec{
		Name:   "my-service",
		Type:   "infra.container_service",
		Config: map[string]any{"image": "gcr.io/test/app:latest"},
	}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID == "" {
		t.Error("expected provider ID")
	}
	if out.Status != "running" {
		t.Errorf("expected status running, got %s", out.Status)
	}
}

func TestCloudRunDriver_Create_Error(t *testing.T) {
	d := &CloudRunDriver{
		Client:    &mockCloudRunClient{createErr: fmt.Errorf("quota exceeded")},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	spec := interfaces.ResourceSpec{Name: "fail-svc", Type: "infra.container_service", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCloudRunDriver_HealthCheck(t *testing.T) {
	d := &CloudRunDriver{
		Client:    &mockCloudRunClient{getResult: map[string]any{"status": "running"}},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	ref := interfaces.ResourceRef{Name: "svc", Type: "infra.container_service", ProviderID: "svc-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}

func TestCloudRunDriver_Update_Success(t *testing.T) {
	d := &CloudRunDriver{Client: &mockCloudRunClient{}, ProjectID: "test-project", Region: "us-central1"}
	ref := interfaces.ResourceRef{Name: "svc", Type: "infra.container_service", ProviderID: "svc-123"}
	spec := interfaces.ResourceSpec{Name: "svc", Type: "infra.container_service", Config: map[string]any{"image": "new:latest"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestCloudRunDriver_Update_Error(t *testing.T) {
	d := &CloudRunDriver{Client: &mockCloudRunClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "svc", Type: "infra.container_service", ProviderID: "svc-123"}
	spec := interfaces.ResourceSpec{Name: "svc", Type: "infra.container_service", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCloudRunDriver_Delete_Success(t *testing.T) {
	d := &CloudRunDriver{Client: &mockCloudRunClient{}, ProjectID: "test-project", Region: "us-central1"}
	ref := interfaces.ResourceRef{Name: "svc", Type: "infra.container_service", ProviderID: "svc-123"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloudRunDriver_Delete_Error(t *testing.T) {
	d := &CloudRunDriver{Client: &mockCloudRunClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "svc", Type: "infra.container_service", ProviderID: "svc-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestCloudRunDriver_Diff_NoChanges(t *testing.T) {
	d := &CloudRunDriver{Client: &mockCloudRunClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "svc", Config: map[string]any{"image": "app:v1"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"image": "app:v1"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestCloudRunDriver_Diff_HasChanges(t *testing.T) {
	d := &CloudRunDriver{Client: &mockCloudRunClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "svc", Config: map[string]any{"image": "app:v2"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"image": "app:v1"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsUpdate {
		t.Error("expected update needed")
	}
	if len(diff.Changes) != 1 {
		t.Errorf("expected 1 change, got %d", len(diff.Changes))
	}
}

func TestCloudRunDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &CloudRunDriver{
		Client:    &mockCloudRunClient{getErr: fmt.Errorf("service unavailable")},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	ref := interfaces.ResourceRef{Name: "svc", Type: "infra.container_service", ProviderID: "svc-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}

func TestCloudRunDriver_Scale(t *testing.T) {
	d := &CloudRunDriver{
		Client:    &mockCloudRunClient{},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	ref := interfaces.ResourceRef{Name: "svc", Type: "infra.container_service", ProviderID: "svc-123"}
	out, err := d.Scale(context.Background(), ref, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}
