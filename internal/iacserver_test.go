// Package internal exercises the gcpIaCServer typed gRPC methods.
// Tests use a real *provider.GCPProvider with no initialized GCP session;
// only methods that do NOT require live GCP credentials are covered here.
// Initialize, Plan, Apply, Destroy, Import, Status test coverage lives in
// provider/provider_test.go (existing suite).
package internal

import (
	"context"
	"testing"

	pb "github.com/GoCodeAlone/workflow/plugin/external/proto"
)

func TestNewIaCServer_NotNil(t *testing.T) {
	s := NewIaCServer()
	if s == nil {
		t.Fatal("NewIaCServer returned nil")
	}
}

func TestGCPIaCServer_Name(t *testing.T) {
	s := NewIaCServer()
	resp, err := s.Name(context.Background(), &pb.NameRequest{})
	if err != nil {
		t.Fatalf("Name: %v", err)
	}
	if resp.GetName() != "gcp" {
		t.Errorf("Name = %q, want %q", resp.GetName(), "gcp")
	}
}

func TestGCPIaCServer_Version(t *testing.T) {
	s := NewIaCServer()
	resp, err := s.Version(context.Background(), &pb.VersionRequest{})
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if resp.GetVersion() == "" {
		t.Error("Version returned empty string")
	}
}

func TestGCPIaCServer_Capabilities_HasContainerService(t *testing.T) {
	s := NewIaCServer()
	resp, err := s.Capabilities(context.Background(), &pb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	found := false
	for _, cap := range resp.GetCapabilities() {
		if cap.GetResourceType() == "infra.container_service" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Capabilities missing infra.container_service")
	}
}

// TestGCPIaCServer_Capabilities_ComputePlanVersionV2 pins the Phase 2
// contract signal: the plugin MUST declare ComputePlanVersion="v2" so
// wfctl v0.54.0+ knows to expect populated ApplyResult.Actions. Per
// workflow#640 Phase 2 + ADR 0040. A typo or accidental drop in
// internal/iacserver.go Capabilities() return literal fails this test.
func TestGCPIaCServer_Capabilities_ComputePlanVersionV2(t *testing.T) {
	s := NewIaCServer()
	resp, err := s.Capabilities(context.Background(), &pb.CapabilitiesRequest{})
	if err != nil {
		t.Fatalf("Capabilities: %v", err)
	}
	if got, want := resp.GetComputePlanVersion(), "v2"; got != want {
		t.Errorf("ComputePlanVersion: got %q, want %q", got, want)
	}
}

func TestGCPIaCServer_Initialize_MissingProjectID(t *testing.T) {
	s := NewIaCServer()
	_, err := s.Initialize(context.Background(), &pb.InitializeRequest{ConfigJson: []byte(`{}`)})
	if err == nil {
		t.Error("Initialize with missing project_id should return error")
	}
}

func TestGCPIaCServer_DetectDrift_Uninitialized(t *testing.T) {
	s := NewIaCServer()
	resp, err := s.DetectDrift(context.Background(), &pb.DetectDriftRequest{})
	if err != nil {
		t.Fatalf("DetectDrift on empty refs: %v", err)
	}
	if len(resp.GetDrifts()) != 0 {
		t.Errorf("DetectDrift with no refs: expected 0 results, got %d", len(resp.GetDrifts()))
	}
}

func TestGCPIaCServer_DetectDriftWithSpecs_DelegatesToDetectDrift(t *testing.T) {
	s := NewIaCServer()
	resp, err := s.DetectDriftWithSpecs(context.Background(), &pb.DetectDriftWithSpecsRequest{})
	if err != nil {
		t.Fatalf("DetectDriftWithSpecs on empty refs: %v", err)
	}
	if len(resp.GetDrifts()) != 0 {
		t.Errorf("DetectDriftWithSpecs with no refs: expected 0 results, got %d", len(resp.GetDrifts()))
	}
}

func TestGCPIaCServer_CompileTimeGuards(t *testing.T) {
	// Confirm the compile-time guards in iacserver.go hold — if they drift,
	// the build fails before this test even runs.
	var _ pb.IaCProviderRequiredServer = (*gcpIaCServer)(nil)
	var _ pb.IaCProviderDriftDetectorServer = (*gcpIaCServer)(nil)
	var _ pb.ResourceDriverServer = (*gcpIaCServer)(nil)
}
