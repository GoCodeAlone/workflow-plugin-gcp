package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockMemorystoreClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockMemorystoreClient) CreateInstance(_ context.Context, _, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "redis-1", nil
}

func (m *mockMemorystoreClient) GetInstance(_ context.Context, _, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"status": "READY"}, nil
}

func (m *mockMemorystoreClient) UpdateInstance(_ context.Context, _, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockMemorystoreClient) DeleteInstance(_ context.Context, _, _, _ string) error {
	return m.deleteErr
}

func TestMemorystoreDriver_Create_Success(t *testing.T) {
	d := &MemorystoreDriver{
		Client:    &mockMemorystoreClient{},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	spec := interfaces.ResourceSpec{
		Name: "my-cache", Type: "infra.cache",
		Config: map[string]any{"memory_size_gb": 5},
	}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "redis-1" {
		t.Errorf("expected redis-1, got %s", out.ProviderID)
	}
}

func TestMemorystoreDriver_Create_Error(t *testing.T) {
	d := &MemorystoreDriver{
		Client:    &mockMemorystoreClient{createErr: fmt.Errorf("quota exceeded")},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.cache", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMemorystoreDriver_HealthCheck(t *testing.T) {
	d := &MemorystoreDriver{
		Client:    &mockMemorystoreClient{getResult: map[string]any{"status": "READY"}},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	ref := interfaces.ResourceRef{Name: "cache", Type: "infra.cache", ProviderID: "redis-1"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}
