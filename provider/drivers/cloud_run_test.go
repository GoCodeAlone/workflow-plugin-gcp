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
