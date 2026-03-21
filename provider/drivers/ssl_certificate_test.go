package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockSSLCertClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockSSLCertClient) CreateCertificate(_ context.Context, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "cert-123", nil
}

func (m *mockSSLCertClient) GetCertificate(_ context.Context, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"status": "ACTIVE"}, nil
}

func (m *mockSSLCertClient) UpdateCertificate(_ context.Context, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockSSLCertClient) DeleteCertificate(_ context.Context, _, _ string) error {
	return m.deleteErr
}

func TestSSLCertificateDriver_Create_Success(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "cert", Type: "infra.certificate", Config: map[string]any{"domains": "example.com"}}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "cert-123" {
		t.Errorf("expected cert-123, got %s", out.ProviderID)
	}
}

func TestSSLCertificateDriver_Create_Error(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{createErr: fmt.Errorf("limit")}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.certificate", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSLCertificateDriver_HealthCheck(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{getResult: map[string]any{"status": "ACTIVE"}}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "cert", Type: "infra.certificate", ProviderID: "cert-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}
