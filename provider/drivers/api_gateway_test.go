package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockAPIGatewayClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockAPIGatewayClient) CreateGateway(_ context.Context, _, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "gw-123", nil
}

func (m *mockAPIGatewayClient) GetGateway(_ context.Context, _, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"status": "running"}, nil
}

func (m *mockAPIGatewayClient) UpdateGateway(_ context.Context, _, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockAPIGatewayClient) DeleteGateway(_ context.Context, _, _, _ string) error {
	return m.deleteErr
}

func TestAPIGatewayDriver_Create_Success(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "gw", Type: "infra.api_gateway", Config: map[string]any{}}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "gw-123" {
		t.Errorf("expected gw-123, got %s", out.ProviderID)
	}
}

func TestAPIGatewayDriver_Create_Error(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{createErr: fmt.Errorf("fail")}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.api_gateway", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAPIGatewayDriver_Update_Success(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "gw", Type: "infra.api_gateway", ProviderID: "gw-123"}
	spec := interfaces.ResourceSpec{Name: "gw", Config: map[string]any{"api_config": "new-config"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestAPIGatewayDriver_Update_Error(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "gw", Type: "infra.api_gateway", ProviderID: "gw-123"}
	spec := interfaces.ResourceSpec{Name: "gw", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAPIGatewayDriver_Delete_Success(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "gw", Type: "infra.api_gateway", ProviderID: "gw-123"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIGatewayDriver_Delete_Error(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "gw", Type: "infra.api_gateway", ProviderID: "gw-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestAPIGatewayDriver_Diff_HasChanges(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "gw", Config: map[string]any{"api_config": "new"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"api_config": "old"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsUpdate {
		t.Error("expected update needed")
	}
}

func TestAPIGatewayDriver_Diff_NoChanges(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "gw", Config: map[string]any{"api_config": "same"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"api_config": "same"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestAPIGatewayDriver_HealthCheck_Healthy(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "gw", Type: "infra.api_gateway", ProviderID: "gw-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}

func TestAPIGatewayDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &APIGatewayDriver{Client: &mockAPIGatewayClient{getErr: fmt.Errorf("gateway not found")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "gw", Type: "infra.api_gateway", ProviderID: "gw-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
