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
