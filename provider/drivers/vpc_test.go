package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockVPCClient struct {
	createNetErr    error
	getNetResult    map[string]any
	getNetErr       error
	deleteNetErr    error
	createSubnetErr error
	getSubnetErr    error
	deleteSubnetErr error
}

func (m *mockVPCClient) CreateNetwork(_ context.Context, _ string, _ map[string]any) (string, error) {
	if m.createNetErr != nil {
		return "", m.createNetErr
	}
	return "vpc-123", nil
}

func (m *mockVPCClient) GetNetwork(_ context.Context, _, _ string) (map[string]any, error) {
	if m.getNetErr != nil {
		return nil, m.getNetErr
	}
	if m.getNetResult != nil {
		return m.getNetResult, nil
	}
	return map[string]any{"network_id": "vpc-123"}, nil
}

func (m *mockVPCClient) DeleteNetwork(_ context.Context, _, _ string) error {
	return m.deleteNetErr
}

func (m *mockVPCClient) CreateSubnet(_ context.Context, _, _ string, _ map[string]any) (string, error) {
	if m.createSubnetErr != nil {
		return "", m.createSubnetErr
	}
	return "subnet-456", nil
}

func (m *mockVPCClient) GetSubnet(_ context.Context, _, _, _ string) (map[string]any, error) {
	if m.getSubnetErr != nil {
		return nil, m.getSubnetErr
	}
	return map[string]any{}, nil
}

func (m *mockVPCClient) DeleteSubnet(_ context.Context, _, _, _ string) error {
	return m.deleteSubnetErr
}

func TestVPCDriver_Create_Success(t *testing.T) {
	d := &VPCDriver{
		Client:    &mockVPCClient{},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	spec := interfaces.ResourceSpec{
		Name: "my-vpc", Type: "infra.vpc",
		Config: map[string]any{"subnet_cidr": "10.0.0.0/24"},
	}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Outputs["network_id"] != "vpc-123" {
		t.Errorf("expected network_id vpc-123, got %v", out.Outputs["network_id"])
	}
	if out.Outputs["subnet_id"] != "subnet-456" {
		t.Errorf("expected subnet_id subnet-456, got %v", out.Outputs["subnet_id"])
	}
}

func TestVPCDriver_Create_NetworkError(t *testing.T) {
	d := &VPCDriver{
		Client:    &mockVPCClient{createNetErr: fmt.Errorf("network error")},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.vpc", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVPCDriver_Delete(t *testing.T) {
	d := &VPCDriver{
		Client:    &mockVPCClient{getNetResult: map[string]any{"subnet_id": "sub-1"}},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	ref := interfaces.ResourceRef{Name: "vpc", Type: "infra.vpc", ProviderID: "vpc-123"}
	err := d.Delete(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
