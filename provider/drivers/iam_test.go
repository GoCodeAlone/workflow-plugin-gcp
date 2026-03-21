package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockIAMClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockIAMClient) CreateRole(_ context.Context, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "customRole1", nil
}

func (m *mockIAMClient) GetRole(_ context.Context, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"role_id": "customRole1"}, nil
}

func (m *mockIAMClient) UpdateRole(_ context.Context, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockIAMClient) DeleteRole(_ context.Context, _, _ string) error {
	return m.deleteErr
}

func TestIAMDriver_Create_Success(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "role", Type: "infra.iam_role", Config: map[string]any{"title": "My Role"}}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "customRole1" {
		t.Errorf("expected customRole1, got %s", out.ProviderID)
	}
}

func TestIAMDriver_Create_Error(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{createErr: fmt.Errorf("permission denied")}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "fail", Type: "infra.iam_role", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIAMDriver_Update_Success(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "role", Type: "infra.iam_role", ProviderID: "customRole1"}
	spec := interfaces.ResourceSpec{Name: "role", Config: map[string]any{"title": "Updated Role"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestIAMDriver_Update_Error(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "role", Type: "infra.iam_role", ProviderID: "customRole1"}
	spec := interfaces.ResourceSpec{Name: "role", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIAMDriver_Delete_Success(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "role", Type: "infra.iam_role", ProviderID: "customRole1"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIAMDriver_Delete_Error(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "role", Type: "infra.iam_role", ProviderID: "customRole1"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestIAMDriver_Diff_HasChanges(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "role", Config: map[string]any{"title": "New Title"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"title": "Old Title"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsUpdate {
		t.Error("expected update needed")
	}
}

func TestIAMDriver_Diff_NoChanges(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{}, ProjectID: "p"}
	spec := interfaces.ResourceSpec{Name: "role", Config: map[string]any{"title": "Same"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"title": "Same"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestIAMDriver_HealthCheck_Healthy(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "role", Type: "infra.iam_role", ProviderID: "customRole1"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hr.Healthy {
		t.Error("expected healthy")
	}
}

func TestIAMDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &IAMDriver{Client: &mockIAMClient{getErr: fmt.Errorf("role not found")}, ProjectID: "p"}
	ref := interfaces.ResourceRef{Name: "role", Type: "infra.iam_role", ProviderID: "customRole1"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}
