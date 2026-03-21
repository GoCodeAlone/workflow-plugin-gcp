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
