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

func TestSSLCertificateDriver_Update_Success(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "cert", Type: "infra.certificate", ProviderID: "cert-123"}
	spec := interfaces.ResourceSpec{Name: "cert", Config: map[string]any{"domains": "new.example.com"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestSSLCertificateDriver_Update_Error(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "cert", Type: "infra.certificate", ProviderID: "cert-123"}
	spec := interfaces.ResourceSpec{Name: "cert", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSLCertificateDriver_Delete_Success(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "cert", Type: "infra.certificate", ProviderID: "cert-123"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSSLCertificateDriver_Delete_Error(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "cert", Type: "infra.certificate", ProviderID: "cert-123"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestSSLCertificateDriver_Diff_NeedsReplace(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "cert", Config: map[string]any{"domains": "new.example.com"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"domains": "old.example.com"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsReplace {
		t.Error("expected needs replace for domains change")
	}
}

func TestSSLCertificateDriver_Diff_NoChanges(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "cert", Config: map[string]any{"domains": "example.com"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"domains": "example.com"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
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

func TestSSLCertificateDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &SSLCertificateDriver{Client: &mockSSLCertClient{getErr: fmt.Errorf("cert not found")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "cert", Type: "infra.certificate", ProviderID: "cert-123"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
