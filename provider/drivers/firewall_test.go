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

func TestFirewallDriver_Update_Success(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "fw", Type: "infra.firewall", ProviderID: "fw-rule-1"}
	spec := interfaces.ResourceSpec{Name: "fw", Config: map[string]any{"priority": 100}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestFirewallDriver_Update_Error(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "fw", Type: "infra.firewall", ProviderID: "fw-rule-1"}
	spec := interfaces.ResourceSpec{Name: "fw", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFirewallDriver_Delete_Success(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "fw", Type: "infra.firewall", ProviderID: "fw-rule-1"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFirewallDriver_Delete_Error(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "fw", Type: "infra.firewall", ProviderID: "fw-rule-1"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestFirewallDriver_Diff_HasChanges(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "fw", Config: map[string]any{"direction": "EGRESS"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"direction": "INGRESS"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsUpdate {
		t.Error("expected update needed")
	}
}

func TestFirewallDriver_Diff_NoChanges(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "fw", Config: map[string]any{"direction": "INGRESS"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"direction": "INGRESS"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestFirewallDriver_HealthCheck_Healthy(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "fw", Type: "infra.firewall", ProviderID: "fw-rule-1"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}

func TestFirewallDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &FirewallDriver{Client: &mockFirewallClient{getErr: fmt.Errorf("rule not found")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "fw", Type: "infra.firewall", ProviderID: "fw-rule-1"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
