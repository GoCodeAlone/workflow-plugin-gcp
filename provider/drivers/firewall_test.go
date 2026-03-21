package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockFirewallClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockFirewallClient) CreateRule(_ context.Context, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "fw-rule-1", nil
}

func (m *mockFirewallClient) GetRule(_ context.Context, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"direction": "INGRESS"}, nil
}

func (m *mockFirewallClient) UpdateRule(_ context.Context, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockFirewallClient) DeleteRule(_ context.Context, _, _ string) error {
	return m.deleteErr
}

func TestFirewallDriver_Create_Success(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "fw", Type: "infra.firewall", Config: map[string]any{"direction": "INGRESS"}}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "fw-rule-1" {
		t.Errorf("expected fw-rule-1, got %s", out.ProviderID)
	}
}

func TestFirewallDriver_Create_Error(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{createErr: fmt.Errorf("denied")}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.firewall", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}
