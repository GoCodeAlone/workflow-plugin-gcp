package drivers

import (
	"context"
	"fmt"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

type mockCloudSQLClient struct {
	createErr error
	getResult map[string]any
	getErr    error
	updateErr error
	deleteErr error
}

func (m *mockCloudSQLClient) CreateInstance(_ context.Context, _ string, _ map[string]any) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return "sql-instance-1", nil
}

func (m *mockCloudSQLClient) GetInstance(_ context.Context, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"status": "RUNNABLE"}, nil
}

func (m *mockCloudSQLClient) UpdateInstance(_ context.Context, _, _ string, _ map[string]any) error {
	return m.updateErr
}

func (m *mockCloudSQLClient) DeleteInstance(_ context.Context, _, _ string) error {
	return m.deleteErr
}

func TestCloudSQLDriver_Create_Success(t *testing.T) {
	d := &CloudSQLDriver{
		Client:    &mockCloudSQLClient{},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	spec := interfaces.ResourceSpec{
		Name: "my-db", Type: "infra.database",
		Config: map[string]any{"tier": "db-n1-standard-2", "database_version": "POSTGRES_15"},
	}
	out, err := d.Create(context.Background(), spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ProviderID != "sql-instance-1" {
		t.Errorf("expected sql-instance-1, got %s", out.ProviderID)
	}
	if out.Outputs["connection_string"] == nil {
		t.Error("expected connection_string in outputs")
	}
}

func TestCloudSQLDriver_Create_Error(t *testing.T) {
	d := &CloudSQLDriver{
		Client:    &mockCloudSQLClient{createErr: fmt.Errorf("api error")},
		ProjectID: "test-project",
		Region:    "us-central1",
	}
	spec := interfaces.ResourceSpec{Name: "fail-db", Type: "infra.database", Config: map[string]any{}}
	_, err := d.Create(context.Background(), spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCloudSQLDriver_Update_Success(t *testing.T) {
	d := &CloudSQLDriver{Client: &mockCloudSQLClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "db", Type: "infra.database", ProviderID: "sql-instance-1"}
	spec := interfaces.ResourceSpec{Name: "db", Config: map[string]any{"tier": "db-n1-standard-4"}}
	out, err := d.Update(context.Background(), ref, spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == nil {
		t.Fatal("expected output")
	}
}

func TestCloudSQLDriver_Update_Error(t *testing.T) {
	d := &CloudSQLDriver{Client: &mockCloudSQLClient{updateErr: fmt.Errorf("update failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "db", Type: "infra.database", ProviderID: "sql-instance-1"}
	spec := interfaces.ResourceSpec{Name: "db", Config: map[string]any{}}
	_, err := d.Update(context.Background(), ref, spec)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCloudSQLDriver_Delete_Success(t *testing.T) {
	d := &CloudSQLDriver{Client: &mockCloudSQLClient{}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "db", Type: "infra.database", ProviderID: "sql-instance-1"}
	if err := d.Delete(context.Background(), ref); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCloudSQLDriver_Delete_Error(t *testing.T) {
	d := &CloudSQLDriver{Client: &mockCloudSQLClient{deleteErr: fmt.Errorf("delete failed")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "db", Type: "infra.database", ProviderID: "sql-instance-1"}
	if err := d.Delete(context.Background(), ref); err == nil {
		t.Fatal("expected error")
	}
}

func TestCloudSQLDriver_Diff_NeedsReplace(t *testing.T) {
	d := &CloudSQLDriver{Client: &mockCloudSQLClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "db", Config: map[string]any{"database_version": "POSTGRES_16"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"database_version": "POSTGRES_15"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !diff.NeedsReplace {
		t.Error("expected needs replace for database_version change")
	}
}

func TestCloudSQLDriver_Diff_NoChanges(t *testing.T) {
	d := &CloudSQLDriver{Client: &mockCloudSQLClient{}, ProjectID: "p", Region: "r"}
	spec := interfaces.ResourceSpec{Name: "db", Config: map[string]any{"tier": "db-f1-micro"}}
	current := &interfaces.ResourceOutput{Outputs: map[string]any{"tier": "db-f1-micro"}}
	diff, err := d.Diff(context.Background(), spec, current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if diff.NeedsUpdate {
		t.Error("expected no update needed")
	}
}

func TestCloudSQLDriver_HealthCheck_Unhealthy(t *testing.T) {
	d := &CloudSQLDriver{Client: &mockCloudSQLClient{getErr: fmt.Errorf("instance not found")}, ProjectID: "p", Region: "r"}
	ref := interfaces.ResourceRef{Name: "db", Type: "infra.database", ProviderID: "sql-instance-1"}
	hr, err := d.HealthCheck(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hr.Healthy {
		t.Error("expected unhealthy")
	}
}

func TestCloudSQLDriver_Scale_NotSupported(t *testing.T) {
	d := &CloudSQLDriver{Client: &mockCloudSQLClient{}, ProjectID: "p", Region: "r"}
	_, err := d.Scale(context.Background(), interfaces.ResourceRef{}, 1)
	if err == nil {
		t.Fatal("expected error for unsupported scale")
	}
}
