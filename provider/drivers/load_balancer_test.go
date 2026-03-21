package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockLBClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockLBClient) Create(_ context.Context, _, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "lb-123", nil
}

func (m *mockLBClient) Get(_ context.Context, _, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"status": "running"}, nil
}

func (m *mockLBClient) Update(_ context.Context, _, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockLBClient) Delete(_ context.Context, _, _, _ string) error {
	return m.deleteErr
}

func TestLoadBalancerDriver_Create_Success(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "lb", Type: "infra.load_balancer", Config: map[string]any{}}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "lb-123" {
		t.Errorf("expected lb-123, got %s", out.ProviderID)
	}
}

func TestLoadBalancerDriver_Create_Error(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{createErr: fmt.Errorf("fail")}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "lb", Type: "infra.load_balancer", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadBalancerDriver_Update_Success(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "lb", Type: "infra.load_balancer", ProviderID: "lb-123"}
	spec := interfaces.ResourceSpec{Name: "lb", Config: map[string]any{"target": "new-target"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestLoadBalancerDriver_Update_Error(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "lb", Type: "infra.load_balancer", ProviderID: "lb-123"}
	spec := interfaces.ResourceSpec{Name: "lb", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadBalancerDriver_Delete_Success(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "lb", Type: "infra.load_balancer", ProviderID: "lb-123"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadBalancerDriver_Delete_Error(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "lb", Type: "infra.load_balancer", ProviderID: "lb-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadBalancerDriver_Diff_HasChanges(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "lb", Config: map[string]any{"target": "new-target"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"target": "old-target"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsUpdate {
		t.Error("expected update needed")
	}
}

func TestLoadBalancerDriver_Diff_NoChanges(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "lb", Config: map[string]any{"target": "same"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"target": "same"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestLoadBalancerDriver_HealthCheck_Healthy(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "lb", Type: "infra.load_balancer", ProviderID: "lb-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}

func TestLoadBalancerDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &LoadBalancerDriver{Client: &mockLBClient{getErr: fmt.Errorf("not found")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "lb", Type: "infra.load_balancer", ProviderID: "lb-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
