package provider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// TrafficTarget represents a Cloud Run traffic routing entry.
type TrafficTarget struct {
	RevisionName   string
	Percent        int
	LatestRevision bool
}

// CloudRunDeployClient defines the Cloud Run operations needed by deploy drivers.
// Wraps the subset of Cloud Run API methods used for deployment lifecycle management.
type CloudRunDeployClient interface {
	GetService(ctx context.Context, projectID, region, serviceID string) (map[string]any, error)
	UpdateService(ctx context.Context, projectID, region, serviceID string, config map[string]any) error
	// GetLatestRevision returns the name of the currently serving revision.
	GetLatestRevision(ctx context.Context, projectID, region, serviceID string) (string, error)
	// CreateRevision deploys a new revision with the given image and initial traffic percent.
	CreateRevision(ctx context.Context, projectID, region, serviceID, image string, trafficPercent int) (string, error)
	// UpdateTraffic updates the traffic split across revisions.
	UpdateTraffic(ctx context.Context, projectID, region, serviceID string, targets []TrafficTarget) error
	// QueryErrorRate queries Cloud Monitoring for the error rate on the given revision (0.0–1.0).
	QueryErrorRate(ctx context.Context, projectID, revisionName string) (float64, error)
}

// ── CloudRunDeployDriver ──────────────────────────────────────────────────────
// Implements github.com/GoCodeAlone/workflow/module.DeployDriver via Cloud Run.
// Rolling deployments are natively handled by Cloud Run: updating the service
// image triggers a new revision that gradually replaces the old one.

// CloudRunDeployDriver implements rolling deployments on Cloud Run.
type CloudRunDeployDriver struct {
	Client     CloudRunDeployClient
	ProjectID  string
	Region     string
	ServiceID  string
	httpClient *http.Client
}

func (d *CloudRunDeployDriver) Update(ctx context.Context, image string) error {
	return d.Client.UpdateService(ctx, d.ProjectID, d.Region, d.ServiceID, map[string]any{
		"image": image,
	})
}

func (d *CloudRunDeployDriver) HealthCheck(ctx context.Context, path string) error {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return fmt.Errorf("cloud run health check: %w", err)
	}
	if status, _ := info["status"].(string); status != "" && status != "running" && status != "READY" {
		return fmt.Errorf("cloud run service %q not ready: status=%s", d.ServiceID, status)
	}
	if path == "" {
		return nil
	}
	serviceURL, _ := info["url"].(string)
	if serviceURL == "" {
		return nil
	}
	client := d.httpClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL+path, nil)
	if err != nil {
		return fmt.Errorf("cloud run health check request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cloud run health check: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("cloud run health check returned %d", resp.StatusCode)
	}
	return nil
}

func (d *CloudRunDeployDriver) CurrentImage(ctx context.Context) (string, error) {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return "", fmt.Errorf("cloud run current image: %w", err)
	}
	image, _ := info["image"].(string)
	return image, nil
}

func (d *CloudRunDeployDriver) ReplicaCount(ctx context.Context) (int, error) {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return 0, fmt.Errorf("cloud run replica count: %w", err)
	}
	if v, ok := info["min_instances"].(int); ok && v > 0 {
		return v, nil
	}
	return 1, nil
}

// ── CloudRunBlueGreenDriver ───────────────────────────────────────────────────
// Implements github.com/GoCodeAlone/workflow/module.BlueGreenDriver via Cloud Run.
// The "green" revision is deployed at 0% traffic, then traffic is atomically
// flipped 100% to the new revision. Old ("blue") revisions are retained by
// Cloud Run but receive no traffic.

// CloudRunBlueGreenDriver implements blue/green deployments using Cloud Run revisions.
type CloudRunBlueGreenDriver struct {
	Client        CloudRunDeployClient
	ProjectID     string
	Region        string
	ServiceID     string
	greenRevision string
}

func (d *CloudRunBlueGreenDriver) Update(ctx context.Context, image string) error {
	return d.Client.UpdateService(ctx, d.ProjectID, d.Region, d.ServiceID, map[string]any{"image": image})
}

func (d *CloudRunBlueGreenDriver) HealthCheck(ctx context.Context, _ string) error {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return fmt.Errorf("cloud run health check: %w", err)
	}
	if status, _ := info["status"].(string); status != "" && status != "running" && status != "READY" {
		return fmt.Errorf("cloud run service %q not ready: status=%s", d.ServiceID, status)
	}
	return nil
}

func (d *CloudRunBlueGreenDriver) CurrentImage(ctx context.Context) (string, error) {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return "", fmt.Errorf("cloud run current image: %w", err)
	}
	image, _ := info["image"].(string)
	return image, nil
}

func (d *CloudRunBlueGreenDriver) ReplicaCount(ctx context.Context) (int, error) {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return 0, fmt.Errorf("cloud run replica count: %w", err)
	}
	if v, ok := info["min_instances"].(int); ok && v > 0 {
		return v, nil
	}
	return 1, nil
}

// CreateGreen deploys a new Cloud Run revision with 0% traffic (green environment).
func (d *CloudRunBlueGreenDriver) CreateGreen(ctx context.Context, image string) error {
	rev, err := d.Client.CreateRevision(ctx, d.ProjectID, d.Region, d.ServiceID, image, 0)
	if err != nil {
		return fmt.Errorf("cloud run create green revision: %w", err)
	}
	d.greenRevision = rev
	return nil
}

// SwitchTraffic routes 100% of traffic to the green revision.
func (d *CloudRunBlueGreenDriver) SwitchTraffic(ctx context.Context) error {
	if d.greenRevision == "" {
		return fmt.Errorf("cloud run: no green revision — call CreateGreen first")
	}
	return d.Client.UpdateTraffic(ctx, d.ProjectID, d.Region, d.ServiceID, []TrafficTarget{
		{RevisionName: d.greenRevision, Percent: 100},
	})
}

// DestroyBlue is a no-op: Cloud Run retains old revisions but they serve no traffic.
func (d *CloudRunBlueGreenDriver) DestroyBlue(_ context.Context) error { return nil }

// GreenEndpoint returns the Cloud Run service URL serving the green revision.
func (d *CloudRunBlueGreenDriver) GreenEndpoint(_ context.Context) (string, error) {
	if d.greenRevision == "" {
		return "", fmt.Errorf("cloud run: no green revision deployed")
	}
	return fmt.Sprintf("https://%s-%s.a.run.app", d.ServiceID, d.ProjectID), nil
}

// ── CloudRunCanaryDriver ──────────────────────────────────────────────────────
// Implements github.com/GoCodeAlone/workflow/module.CanaryDriver via Cloud Run.
// Cloud Run's first-class traffic splitting (TrafficTarget per revision with
// percent) makes progressive canary rollout straightforward.

// CloudRunCanaryDriver implements canary deployments using Cloud Run traffic splitting.
type CloudRunCanaryDriver struct {
	Client         CloudRunDeployClient
	ProjectID      string
	Region         string
	ServiceID      string
	ErrorThreshold float64 // error rate threshold for metric gates (0.0–1.0); default 0.01
	canaryRevision string
	stableRevision string
}

func (d *CloudRunCanaryDriver) Update(ctx context.Context, image string) error {
	return d.Client.UpdateService(ctx, d.ProjectID, d.Region, d.ServiceID, map[string]any{"image": image})
}

func (d *CloudRunCanaryDriver) HealthCheck(ctx context.Context, _ string) error {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return fmt.Errorf("cloud run health check: %w", err)
	}
	if status, _ := info["status"].(string); status != "" && status != "running" && status != "READY" {
		return fmt.Errorf("cloud run service %q not ready: status=%s", d.ServiceID, status)
	}
	return nil
}

func (d *CloudRunCanaryDriver) CurrentImage(ctx context.Context) (string, error) {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return "", fmt.Errorf("cloud run current image: %w", err)
	}
	image, _ := info["image"].(string)
	return image, nil
}

func (d *CloudRunCanaryDriver) ReplicaCount(ctx context.Context) (int, error) {
	info, err := d.Client.GetService(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return 0, fmt.Errorf("cloud run replica count: %w", err)
	}
	if v, ok := info["min_instances"].(int); ok && v > 0 {
		return v, nil
	}
	return 1, nil
}

// CreateCanary deploys a new canary revision with 0% traffic, preserving the
// current stable revision name for later rollback or promotion.
func (d *CloudRunCanaryDriver) CreateCanary(ctx context.Context, image string) error {
	stable, err := d.Client.GetLatestRevision(ctx, d.ProjectID, d.Region, d.ServiceID)
	if err != nil {
		return fmt.Errorf("cloud run get stable revision: %w", err)
	}
	d.stableRevision = stable

	canary, err := d.Client.CreateRevision(ctx, d.ProjectID, d.Region, d.ServiceID, image, 0)
	if err != nil {
		return fmt.Errorf("cloud run create canary revision: %w", err)
	}
	d.canaryRevision = canary
	return nil
}

// RoutePercent splits traffic so percent% goes to the canary and (100-percent)% to stable.
func (d *CloudRunCanaryDriver) RoutePercent(ctx context.Context, percent int) error {
	return d.Client.UpdateTraffic(ctx, d.ProjectID, d.Region, d.ServiceID, []TrafficTarget{
		{RevisionName: d.canaryRevision, Percent: percent},
		{RevisionName: d.stableRevision, Percent: 100 - percent},
	})
}

// CheckMetricGate queries Cloud Monitoring error rate on the canary revision.
// Returns an error if the rate exceeds ErrorThreshold (default 1%).
func (d *CloudRunCanaryDriver) CheckMetricGate(ctx context.Context, gate string) error {
	threshold := d.ErrorThreshold
	if threshold == 0 {
		threshold = 0.01
	}
	rate, err := d.Client.QueryErrorRate(ctx, d.ProjectID, d.canaryRevision)
	if err != nil {
		return fmt.Errorf("cloud run metric gate %q: query error rate: %w", gate, err)
	}
	if rate > threshold {
		return fmt.Errorf("cloud run metric gate %q: error rate %.2f%% exceeds threshold %.2f%%",
			gate, rate*100, threshold*100)
	}
	return nil
}

// PromoteCanary routes 100% of traffic to the canary revision.
func (d *CloudRunCanaryDriver) PromoteCanary(ctx context.Context) error {
	return d.Client.UpdateTraffic(ctx, d.ProjectID, d.Region, d.ServiceID, []TrafficTarget{
		{RevisionName: d.canaryRevision, Percent: 100},
	})
}

// DestroyCanary rolls traffic back to the stable revision (100%).
// The canary revision is retained by Cloud Run but serves no traffic.
func (d *CloudRunCanaryDriver) DestroyCanary(ctx context.Context) error {
	return d.Client.UpdateTraffic(ctx, d.ProjectID, d.Region, d.ServiceID, []TrafficTarget{
		{RevisionName: d.stableRevision, Percent: 100},
	})
}
