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

func TestVPCDriver_Update_Success(t *testing.T) {
	d := &VPCDriver{Client: &mockVPCClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "vpc", Type: "infra.vpc", ProviderID: "vpc-123"}
	spec := interfaces.ResourceSpec{Name: "vpc", Config: map[string]any{"subnet_id": "sub-1", "subnet_cidr": "10.0.1.0/24"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestVPCDriver_Update_SubnetDeleteError(t *testing.T) {
	d := &VPCDriver{Client: &mockVPCClient{deleteSubnetErr: fmt.Errorf("subnet delete failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "vpc", Type: "infra.vpc", ProviderID: "vpc-123"}
	spec := interfaces.ResourceSpec{Name: "vpc", Config: map[string]any{"subnet_id": "sub-1", "subnet_cidr": "10.0.1.0/24"}}
	_, err := d.Update(context.Background(), ref, spec)
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

func TestVPCDriver_Delete_Error(t *testing.T) {
	d := &VPCDriver{Client: &mockVPCClient{deleteNetErr: fmt.Errorf("network delete failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "vpc", Type: "infra.vpc", ProviderID: "vpc-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestVPCDriver_Diff_NeedsReplace(t *testing.T) {
	d := &VPCDriver{Client: &mockVPCClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "vpc", Config: map[string]any{"subnet_cidr": "10.0.1.0/24"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"subnet_cidr": "10.0.0.0/24"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsReplace {
		t.Error("expected needs replace for subnet_cidr change")
	}
}

func TestVPCDriver_Diff_NoChanges(t *testing.T) {
	d := &VPCDriver{Client: &mockVPCClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "vpc", Config: map[string]any{"subnet_cidr": "10.0.0.0/24"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"subnet_cidr": "10.0.0.0/24"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestVPCDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &VPCDriver{Client: &mockVPCClient{getNetErr: fmt.Errorf("network not found")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "vpc", Type: "infra.vpc", ProviderID: "vpc-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
