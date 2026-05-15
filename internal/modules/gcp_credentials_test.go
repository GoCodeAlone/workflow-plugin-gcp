package modules

import (
	"context"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/credref"
)

func TestGCPCredentialsProvider_ModuleTypes(t *testing.T) {
	p := NewGCPCredentialsProvider()
	types := p.ModuleTypes()
	if len(types) != 1 || types[0] != "gcp.credentials" {
		t.Errorf("ModuleTypes = %v, want [gcp.credentials]", types)
	}
}

func TestGCPCredentialsProvider_CreateModule_RegistersCredentials(t *testing.T) {
	t.Cleanup(credref.Reset)
	p := NewGCPCredentialsProvider()

	cfg := map[string]any{
		"credentials": map[string]any{
			"projectId":          "my-project",
			"serviceAccountJson": `{"type":"service_account","project_id":"my-project"}`,
		},
	}
	inst, err := p.CreateModule("gcp.credentials", "default-creds", cfg)
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	if inst == nil {
		t.Fatal("CreateModule returned nil instance")
	}

	got, ok := credref.Resolve("default-creds")
	if !ok {
		t.Fatal("credref.Resolve(default-creds): not found")
	}
	if got.ProjectID != "my-project" {
		t.Errorf("ProjectID = %q, want my-project", got.ProjectID)
	}
	if string(got.ServiceAccountJSON) != `{"type":"service_account","project_id":"my-project"}` {
		t.Errorf("ServiceAccountJSON not preserved: %s", got.ServiceAccountJSON)
	}
}

func TestGCPCredentialsProvider_CreateModule_TopLevelProjectIDFallback(t *testing.T) {
	t.Cleanup(credref.Reset)
	p := NewGCPCredentialsProvider()
	cfg := map[string]any{"projectId": "fallback-project"}
	if _, err := p.CreateModule("gcp.credentials", "top-only", cfg); err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	got, ok := credref.Resolve("top-only")
	if !ok {
		t.Fatal("not registered")
	}
	if got.ProjectID != "fallback-project" {
		t.Errorf("ProjectID = %q, want fallback-project (top-level fallback)", got.ProjectID)
	}
}

func TestGCPCredentialsProvider_CreateModule_NestedProjectIDWinsOverTopLevel(t *testing.T) {
	t.Cleanup(credref.Reset)
	p := NewGCPCredentialsProvider()
	cfg := map[string]any{
		"projectId": "top",
		"credentials": map[string]any{
			"projectId": "nested",
		},
	}
	if _, err := p.CreateModule("gcp.credentials", "winner", cfg); err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	got, _ := credref.Resolve("winner")
	if got.ProjectID != "nested" {
		t.Errorf("ProjectID = %q, want nested (credentials.projectId beats top-level)", got.ProjectID)
	}
}

func TestGCPCredentialsProvider_CreateModule_DuplicateNameErrors(t *testing.T) {
	t.Cleanup(credref.Reset)
	p := NewGCPCredentialsProvider()
	cfg := map[string]any{"credentials": map[string]any{"projectId": "p"}}
	if _, err := p.CreateModule("gcp.credentials", "dup", cfg); err != nil {
		t.Fatalf("first CreateModule: %v", err)
	}
	if _, err := p.CreateModule("gcp.credentials", "dup", cfg); err == nil {
		t.Fatal("expected duplicate-name error on second CreateModule with same name")
	}
}

func TestGCPCredentialsInstance_LifecycleIsNoOp(t *testing.T) {
	t.Cleanup(credref.Reset)
	p := NewGCPCredentialsProvider()
	inst, err := p.CreateModule("gcp.credentials", "lifecycle", map[string]any{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	if err := inst.Init(); err != nil {
		t.Errorf("Init: %v", err)
	}
	if err := inst.Start(context.Background()); err != nil {
		t.Errorf("Start: %v", err)
	}
	if err := inst.Stop(context.Background()); err != nil {
		t.Errorf("Stop: %v", err)
	}
}
