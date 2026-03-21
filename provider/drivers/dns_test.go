package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockDNSClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockDNSClient) CreateManagedZone(_ context.Context, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "zone-123", nil
}

func (m *mockDNSClient) GetManagedZone(_ context.Context, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"zone_id": "zone-123"}, nil
}

func (m *mockDNSClient) UpdateManagedZone(_ context.Context, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockDNSClient) DeleteManagedZone(_ context.Context, _, _ string) error {
	return m.deleteErr
}

func TestDNSDriver_Create_Success(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "myzone", Type: "infra.dns", Config: map[string]any{"dns_name": "example.com."}}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "zone-123" {
		t.Errorf("expected zone-123, got %s", out.ProviderID)
	}
}

func TestDNSDriver_Create_Error(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{createErr: fmt.Errorf("zone exists")}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.dns", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDNSDriver_Update_Success(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "zone", Type: "infra.dns", ProviderID: "zone-123"}
	spec := interfaces.ResourceSpec{Name: "zone", Config: map[string]any{"description": "updated"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestDNSDriver_Update_Error(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "zone", Type: "infra.dns", ProviderID: "zone-123"}
	spec := interfaces.ResourceSpec{Name: "zone", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDNSDriver_Delete_Success(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "zone", Type: "infra.dns", ProviderID: "zone-123"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDNSDriver_Delete_Error(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "zone", Type: "infra.dns", ProviderID: "zone-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestDNSDriver_Diff_HasChanges(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "zone", Config: map[string]any{"dns_name": "new.example.com."}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"dns_name": "old.example.com."}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsUpdate {
		t.Error("expected update needed")
	}
}

func TestDNSDriver_Diff_NoChanges(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "zone", Config: map[string]any{"dns_name": "example.com."}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"dns_name": "example.com."}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestDNSDriver_HealthCheck_Healthy(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "zone", Type: "infra.dns", ProviderID: "zone-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}

func TestDNSDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &DNSDriver{Client: &mockDNSClient{getErr: fmt.Errorf("zone not found")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "zone", Type: "infra.dns", ProviderID: "zone-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
