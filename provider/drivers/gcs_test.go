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
