package provider

import (
	"context"
	"fmt"
	"testing"
)

// ── mock ─────────────────────────────────────────────────────────────────────

type mockDeployClient struct {
	getResult         map[string]any
	getErr            error
	updateErr         error
	latestRevision    string
	latestRevisionErr error
	createdRevision   string
	createRevErr      error
	updateTrafficErr  error
	errorRate         float64
	errorRateErr      error

	// recorded calls
	lastUpdateConfig map[string]any
	lastTraffic      []TrafficTarget
}

func (m *mockDeployClient) GetService(_ context.Context, _, _, _ string) (map[string]any, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.getResult != nil {
		return m.getResult, nil
	}
	return map[string]any{"status": "running", "image": "app:stable"}, nil
}

func (m *mockDeployClient) UpdateService(_ context.Context, _, _, _ string, config map[string]any) error {
	m.lastUpdateConfig = config
	return m.updateErr
}

func (m *mockDeployClient) GetLatestRevision(_ context.Context, _, _, _ string) (string, error) {
	if m.latestRevisionErr != nil {
		return "", m.latestRevisionErr
	}
	if m.latestRevision != "" {
		return m.latestRevision, nil
	}
	return "stable-rev-001", nil
}

func (m *mockDeployClient) CreateRevision(_ context.Context, _, _, _, _ string, _ int) (string, error) {
	if m.createRevErr != nil {
		return "", m.createRevErr
	}
	if m.createdRevision != "" {
		return m.createdRevision, nil
	}
	return "new-rev-001", nil
}

func (m *mockDeployClient) UpdateTraffic(_ context.Context, _, _, _ string, targets []TrafficTarget) error {
	m.lastTraffic = targets
	return m.updateTrafficErr
}

func (m *mockDeployClient) QueryErrorRate(_ context.Context, _, _ string) (float64, error) {
	if m.errorRateErr != nil {
		return 0, m.errorRateErr
	}
	return m.errorRate, nil
}

// ── CloudRunDeployDriver (rolling) ───────────────────────────────────────────

func TestDeployDriver_Update_Success(t *testing.T) {
	m := &mockDeployClient{}
	d := &CloudRunDeployDriver{Client: m, ProjectID: "proj", Region: "us-central1", ServiceID: "svc"}
	if err := d.Update(context.Background(), "app:v2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.lastUpdateConfig["image"] != "app:v2" {
		t.Errorf("expected image app:v2 in update config, got %v", m.lastUpdateConfig)
	}
}

func TestDeployDriver_Update_Error(t *testing.T) {
	m := &mockDeployClient{updateErr: fmt.Errorf("quota exceeded")}
	d := &CloudRunDeployDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.Update(context.Background(), "app:v2"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeployDriver_HealthCheck_Ready(t *testing.T) {
	m := &mockDeployClient{getResult: map[string]any{"status": "READY"}}
	d := &CloudRunDeployDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.HealthCheck(context.Background(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeployDriver_HealthCheck_NotReady(t *testing.T) {
	m := &mockDeployClient{getResult: map[string]any{"status": "FAILED"}}
	d := &CloudRunDeployDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.HealthCheck(context.Background(), ""); err == nil {
		t.Fatal("expected error for FAILED status")
	}
}

func TestDeployDriver_HealthCheck_GetError(t *testing.T) {
	m := &mockDeployClient{getErr: fmt.Errorf("not found")}
	d := &CloudRunDeployDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.HealthCheck(context.Background(), "/health"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeployDriver_CurrentImage(t *testing.T) {
	m := &mockDeployClient{getResult: map[string]any{"status": "running", "image": "app:v1"}}
	d := &CloudRunDeployDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	img, err := d.CurrentImage(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if img != "app:v1" {
		t.Errorf("expected app:v1, got %s", img)
	}
}

func TestDeployDriver_ReplicaCount_Default(t *testing.T) {
	m := &mockDeployClient{}
	d := &CloudRunDeployDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	n, err := d.ReplicaCount(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("expected default 1, got %d", n)
	}
}

func TestDeployDriver_ReplicaCount_FromConfig(t *testing.T) {
	m := &mockDeployClient{getResult: map[string]any{"min_instances": 3}}
	d := &CloudRunDeployDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	n, err := d.ReplicaCount(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

// ── CloudRunBlueGreenDriver ───────────────────────────────────────────────────

func TestBlueGreenDriver_FullLifecycle(t *testing.T) {
	m := &mockDeployClient{createdRevision: "green-rev-001"}
	d := &CloudRunBlueGreenDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}

	// CreateGreen
	if err := d.CreateGreen(context.Background(), "app:v2"); err != nil {
		t.Fatalf("CreateGreen: %v", err)
	}
	if d.greenRevision != "green-rev-001" {
		t.Errorf("expected green revision green-rev-001, got %s", d.greenRevision)
	}

	// HealthCheck
	if err := d.HealthCheck(context.Background(), ""); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}

	// GreenEndpoint
	endpoint, err := d.GreenEndpoint(context.Background())
	if err != nil {
		t.Fatalf("GreenEndpoint: %v", err)
	}
	if endpoint == "" {
		t.Error("expected non-empty endpoint")
	}

	// SwitchTraffic
	if err := d.SwitchTraffic(context.Background()); err != nil {
		t.Fatalf("SwitchTraffic: %v", err)
	}
	if len(m.lastTraffic) != 1 || m.lastTraffic[0].RevisionName != "green-rev-001" || m.lastTraffic[0].Percent != 100 {
		t.Errorf("unexpected traffic targets: %+v", m.lastTraffic)
	}

	// DestroyBlue (no-op)
	if err := d.DestroyBlue(context.Background()); err != nil {
		t.Fatalf("DestroyBlue: %v", err)
	}
}

func TestBlueGreenDriver_SwitchTraffic_NoGreenRevision(t *testing.T) {
	m := &mockDeployClient{}
	d := &CloudRunBlueGreenDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.SwitchTraffic(context.Background()); err == nil {
		t.Fatal("expected error when no green revision set")
	}
}

func TestBlueGreenDriver_GreenEndpoint_NoRevision(t *testing.T) {
	d := &CloudRunBlueGreenDriver{}
	if _, err := d.GreenEndpoint(context.Background()); err == nil {
		t.Fatal("expected error when no green revision")
	}
}

func TestBlueGreenDriver_CreateGreen_Error(t *testing.T) {
	m := &mockDeployClient{createRevErr: fmt.Errorf("revision limit reached")}
	d := &CloudRunBlueGreenDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.CreateGreen(context.Background(), "app:v2"); err == nil {
		t.Fatal("expected error")
	}
}

func TestBlueGreenDriver_HealthCheck_NotReady(t *testing.T) {
	m := &mockDeployClient{getResult: map[string]any{"status": "FAILED"}}
	d := &CloudRunBlueGreenDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.HealthCheck(context.Background(), ""); err == nil {
		t.Fatal("expected error for FAILED status")
	}
}

// ── CloudRunCanaryDriver ──────────────────────────────────────────────────────

func TestCanaryDriver_ProgressiveTraffic(t *testing.T) {
	m := &mockDeployClient{
		latestRevision:  "stable-rev-001",
		createdRevision: "canary-rev-001",
	}
	d := &CloudRunCanaryDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}

	// CreateCanary
	if err := d.CreateCanary(context.Background(), "app:canary"); err != nil {
		t.Fatalf("CreateCanary: %v", err)
	}
	if d.stableRevision != "stable-rev-001" {
		t.Errorf("expected stable-rev-001, got %s", d.stableRevision)
	}
	if d.canaryRevision != "canary-rev-001" {
		t.Errorf("expected canary-rev-001, got %s", d.canaryRevision)
	}

	// RoutePercent 10%
	if err := d.RoutePercent(context.Background(), 10); err != nil {
		t.Fatalf("RoutePercent(10): %v", err)
	}
	assertTraffic(t, m.lastTraffic, "canary-rev-001", 10, "stable-rev-001", 90)

	// RoutePercent 50%
	if err := d.RoutePercent(context.Background(), 50); err != nil {
		t.Fatalf("RoutePercent(50): %v", err)
	}
	assertTraffic(t, m.lastTraffic, "canary-rev-001", 50, "stable-rev-001", 50)

	// PromoteCanary
	if err := d.PromoteCanary(context.Background()); err != nil {
		t.Fatalf("PromoteCanary: %v", err)
	}
	if len(m.lastTraffic) != 1 || m.lastTraffic[0].RevisionName != "canary-rev-001" || m.lastTraffic[0].Percent != 100 {
		t.Errorf("unexpected promote traffic: %+v", m.lastTraffic)
	}
}

func TestCanaryDriver_DestroyCanary(t *testing.T) {
	m := &mockDeployClient{
		latestRevision:  "stable-rev-001",
		createdRevision: "canary-rev-001",
	}
	d := &CloudRunCanaryDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.CreateCanary(context.Background(), "app:canary"); err != nil {
		t.Fatalf("CreateCanary: %v", err)
	}
	if err := d.DestroyCanary(context.Background()); err != nil {
		t.Fatalf("DestroyCanary: %v", err)
	}
	if len(m.lastTraffic) != 1 || m.lastTraffic[0].RevisionName != "stable-rev-001" || m.lastTraffic[0].Percent != 100 {
		t.Errorf("unexpected rollback traffic: %+v", m.lastTraffic)
	}
}

func TestCanaryDriver_CheckMetricGate_Pass(t *testing.T) {
	m := &mockDeployClient{createdRevision: "canary-rev-001", errorRate: 0.005}
	d := &CloudRunCanaryDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc", ErrorThreshold: 0.01}
	_ = d.CreateCanary(context.Background(), "app:canary")
	if err := d.CheckMetricGate(context.Background(), "error-rate"); err != nil {
		t.Fatalf("expected gate to pass: %v", err)
	}
}

func TestCanaryDriver_CheckMetricGate_Fail(t *testing.T) {
	m := &mockDeployClient{createdRevision: "canary-rev-001", errorRate: 0.05}
	d := &CloudRunCanaryDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc", ErrorThreshold: 0.01}
	_ = d.CreateCanary(context.Background(), "app:canary")
	if err := d.CheckMetricGate(context.Background(), "error-rate"); err == nil {
		t.Fatal("expected metric gate to fail")
	}
}

func TestCanaryDriver_CheckMetricGate_QueryError(t *testing.T) {
	m := &mockDeployClient{createdRevision: "canary-rev-001", errorRateErr: fmt.Errorf("monitoring unavailable")}
	d := &CloudRunCanaryDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	_ = d.CreateCanary(context.Background(), "app:canary")
	if err := d.CheckMetricGate(context.Background(), "error-rate"); err == nil {
		t.Fatal("expected error from metric query failure")
	}
}

func TestCanaryDriver_CreateCanary_GetLatestRevisionError(t *testing.T) {
	m := &mockDeployClient{latestRevisionErr: fmt.Errorf("service not found")}
	d := &CloudRunCanaryDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.CreateCanary(context.Background(), "app:canary"); err == nil {
		t.Fatal("expected error when stable revision lookup fails")
	}
}

func TestCanaryDriver_CreateCanary_CreateRevisionError(t *testing.T) {
	m := &mockDeployClient{createRevErr: fmt.Errorf("image not found")}
	d := &CloudRunCanaryDriver{Client: m, ProjectID: "proj", Region: "r", ServiceID: "svc"}
	if err := d.CreateCanary(context.Background(), "app:canary"); err == nil {
		t.Fatal("expected error when revision creation fails")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertTraffic(t *testing.T, targets []TrafficTarget, rev1 string, pct1 int, rev2 string, pct2 int) {
	t.Helper()
	if len(targets) != 2 {
		t.Fatalf("expected 2 traffic targets, got %d: %+v", len(targets), targets)
	}
	byRev := make(map[string]int, 2)
	for _, tgt := range targets {
		byRev[tgt.RevisionName] = tgt.Percent
	}
	if byRev[rev1] != pct1 {
		t.Errorf("expected %s=%d%%, got %d%%", rev1, pct1, byRev[rev1])
	}
	if byRev[rev2] != pct2 {
		t.Errorf("expected %s=%d%%, got %d%%", rev2, pct2, byRev[rev2])
	}
}
