package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockGCSClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockGCSClient) CreateBucket(_ context.Context, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "my-bucket-123", nil
}

func (m *mockGCSClient) GetBucket(_ context.Context, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"bucket_id": "my-bucket-123"}, nil
}

func (m *mockGCSClient) UpdateBucket(_ context.Context, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockGCSClient) DeleteBucket(_ context.Context, _ string) error {
	return m.deleteErr
}

func TestGCSDriver_Create_Success(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "bucket", Type: "infra.storage", Config: map[string]any{"location": "US"}}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Outputs["url"] != "gs://my-bucket-123" {
		t.Errorf("expected gs://my-bucket-123, got %v", out.Outputs["url"])
	}
}

func TestGCSDriver_Create_Error(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{createErr: fmt.Errorf("bucket exists")}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.storage", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGCSDriver_Update_Success(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "bucket", Type: "infra.storage", ProviderID: "my-bucket-123"}
	spec := interfaces.ResourceSpec{Name: "bucket", Config: map[string]any{"storage_class": "NEARLINE"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestGCSDriver_Update_Error(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "bucket", Type: "infra.storage", ProviderID: "my-bucket-123"}
	spec := interfaces.ResourceSpec{Name: "bucket", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGCSDriver_Delete_Success(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "bucket", Type: "infra.storage", ProviderID: "my-bucket-123"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGCSDriver_Delete_Error(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "bucket", Type: "infra.storage", ProviderID: "my-bucket-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestGCSDriver_Diff_NeedsReplace(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "bucket", Config: map[string]any{"location": "EU"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"location": "US"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsReplace {
		t.Error("expected needs replace for location change")
	}
}

func TestGCSDriver_Diff_NoChanges(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "bucket", Config: map[string]any{"location": "US"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"location": "US"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestGCSDriver_HealthCheck_Healthy(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "bucket", Type: "infra.storage", ProviderID: "my-bucket-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}

func TestGCSDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &GCSDriver{Client: &mockGCSClient{getErr: fmt.Errorf("bucket not found")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "bucket", Type: "infra.storage", ProviderID: "my-bucket-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
