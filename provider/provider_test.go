package provider

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/workflow/interfaces"
)

func TestGCPProvider_Initialize(t *testing.T) {
	p := New()
	err := p.Initialize(context.Background(), map[string]any{
		"project_id": "test-project",
		"region":     "europe-west1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.projectID != "test-project" {
		t.Errorf("expected test-project, got %s", p.projectID)
	}
	if p.region != "europe-west1" {
		t.Errorf("expected europe-west1, got %s", p.region)
	}
}

func TestGCPProvider_Initialize_MissingProjectID(t *testing.T) {
	p := New()
	err := p.Initialize(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing project_id")
	}
}

func TestGCPProvider_Capabilities(t *testing.T) {
	p := New()
	caps := p.Capabilities()
	if len(caps) != 13 {
		t.Errorf("expected 13 capabilities, got %d", len(caps))
	}
}

func TestGCPProvider_ResourceDriver_Unknown(t *testing.T) {
	p := New()
	_ = p.Initialize(context.Background(), map[string]any{"project_id": "p"})
	_, err := p.ResourceDriver("infra.nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown resource type")
	}
}

func TestGCPProvider_ResolveSizing(t *testing.T) {
	p := New()
	sizing, err := p.ResolveSizing("infra.database", interfaces.SizeM, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sizing.InstanceType != "db-custom-2-8192" {
		t.Errorf("expected db-custom-2-8192, got %s", sizing.InstanceType)
	}
}

func TestGCPProvider_ResolveSizing_WithHints(t *testing.T) {
	p := New()
	sizing, err := p.ResolveSizing("infra.container_service", interfaces.SizeS, &interfaces.ResourceHints{
		CPU: "2000m",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sizing.Specs["cpu"] != "2000m" {
		t.Errorf("expected cpu override 2000m, got %v", sizing.Specs["cpu"])
	}
}

func TestGCPProvider_ResolveSizing_UnknownType(t *testing.T) {
	p := New()
	_, err := p.ResolveSizing("infra.nonexistent", interfaces.SizeM, nil)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestGCPProvider_NameVersion(t *testing.T) {
	p := New()
	if p.Name() != "gcp" {
		t.Errorf("expected gcp, got %s", p.Name())
	}
	if p.Version() == "" {
		t.Errorf("expected non-empty Version, got empty — build-time ldflags injection missing")
	}
}

func TestGCPProvider_Plan_CreateAction(t *testing.T) {
	p := New()
	_ = p.Initialize(context.Background(), map[string]any{"project_id": "test"})

	// Inject a mock driver for container_service.
	p.SetDriver("infra.container_service", &mockDriver{})

	desired := []interfaces.ResourceSpec{
		{Name: "svc1", Type: "infra.container_service", Config: map[string]any{"image": "app:v1"}},
	}
	plan, err := p.Plan(context.Background(), desired, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(plan.Actions))
	}
	if plan.Actions[0].Action != "create" {
		t.Errorf("expected create action, got %s", plan.Actions[0].Action)
	}
}

func TestGCPProvider_Close(t *testing.T) {
	p := New()
	if err := p.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// mockDriver is a minimal ResourceDriver for provider-level tests.
type mockDriver struct{}

func (m *mockDriver) Create(_ context.Context, spec interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	return &interfaces.ResourceOutput{Name: spec.Name, Type: spec.Type, ProviderID: "mock-id", Status: "running"}, nil
}
func (m *mockDriver) Read(_ context.Context, ref interfaces.ResourceRef) (*interfaces.ResourceOutput, error) {
	return &interfaces.ResourceOutput{Name: ref.Name, Type: ref.Type, ProviderID: ref.ProviderID, Status: "running", Outputs: map[string]any{}}, nil
}
func (m *mockDriver) Update(_ context.Context, ref interfaces.ResourceRef, _ interfaces.ResourceSpec) (*interfaces.ResourceOutput, error) {
	return &interfaces.ResourceOutput{Name: ref.Name, Type: ref.Type, ProviderID: ref.ProviderID, Status: "running"}, nil
}
func (m *mockDriver) Delete(_ context.Context, _ interfaces.ResourceRef) error { return nil }
func (m *mockDriver) Diff(_ context.Context, _ interfaces.ResourceSpec, _ *interfaces.ResourceOutput) (*interfaces.DiffResult, error) {
	return &interfaces.DiffResult{}, nil
}
func (m *mockDriver) HealthCheck(_ context.Context, _ interfaces.ResourceRef) (*interfaces.HealthResult, error) {
	return &interfaces.HealthResult{Healthy: true}, nil
}
func (m *mockDriver) Scale(_ context.Context, _ interfaces.ResourceRef, _ int) (*interfaces.ResourceOutput, error) {
	return nil, nil
}
func (m *mockDriver) SensitiveKeys() []string { return nil }

func TestGCPProvider_BootstrapStateBackend_NoOp(t *testing.T) {
	p := New()
	res, err := p.BootstrapStateBackend(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != nil {
		t.Errorf("expected nil result, got %+v", res)
	}
}

func TestGCPProvider_SupportedCanonicalKeys_FullSet(t *testing.T) {
	p := New()
	got := p.SupportedCanonicalKeys()
	want := interfaces.CanonicalKeys()
	if len(got) != len(want) {
		t.Fatalf("expected %d keys, got %d", len(want), len(got))
	}
	gotSet := make(map[string]bool, len(got))
	for _, k := range got {
		gotSet[k] = true
	}
	for _, k := range want {
		if !gotSet[k] {
			t.Errorf("missing canonical key: %s", k)
		}
	}
}
