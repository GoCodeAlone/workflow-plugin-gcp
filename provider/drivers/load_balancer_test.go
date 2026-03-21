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
