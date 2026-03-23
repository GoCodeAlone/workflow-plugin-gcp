package main_test

import (
	"testing"

	"github.com/GoCodeAlone/workflow/wftest"
)

// TestIntegration_GCPDeployCloudRun tests a pipeline that deploys a Cloud Run service.
func TestIntegration_GCPDeployCloudRun(t *testing.T) {
	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  deploy-cloud-run:
    steps:
      - name: deploy
        type: step.gcp_deploy
        config:
          resource_type: infra.container_service
          project: my-project
          region: us-central1
          image: gcr.io/my-project/my-service:v1
`),
		wftest.MockStep("step.gcp_deploy", wftest.Returns(map[string]any{
			"service_url": "https://my-service-xyz.a.run.app",
			"status":      "deployed",
			"revision":    "my-service-00001-abc",
		})),
	)

	result := h.ExecutePipeline("deploy-cloud-run", nil)
	if result.Error != nil {
		t.Fatalf("pipeline failed: %v", result.Error)
	}
	if !result.StepExecuted("deploy") {
		t.Error("deploy step should have executed")
	}

	output := result.StepOutput("deploy")
	if output["status"] != "deployed" {
		t.Errorf("expected status=deployed, got %v", output["status"])
	}
	if output["service_url"] != "https://my-service-xyz.a.run.app" {
		t.Errorf("expected service_url to be set, got %v", output["service_url"])
	}
	if output["revision"] != "my-service-00001-abc" {
		t.Errorf("expected revision=my-service-00001-abc, got %v", output["revision"])
	}
}

// TestIntegration_GCPProvisionGKE tests a pipeline that provisions a GKE cluster.
func TestIntegration_GCPProvisionGKE(t *testing.T) {
	rec := wftest.RecordStep("step.gcp_provision_gke")
	rec.WithOutput(map[string]any{
		"cluster_name":     "prod-cluster",
		"endpoint":         "https://34.120.0.1",
		"node_count":       3,
		"status":           "RUNNING",
		"kubeconfig_secret": "prod-cluster-kubeconfig",
	})

	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  provision-gke:
    steps:
      - name: create-cluster
        type: step.gcp_provision_gke
        config:
          resource_type: infra.k8s_cluster
          project: my-project
          zone: us-central1-a
          node_count: 3
          machine_type: e2-standard-4
`),
		rec,
	)

	result := h.ExecutePipeline("provision-gke", nil)
	if result.Error != nil {
		t.Fatalf("pipeline failed: %v", result.Error)
	}
	if !result.StepExecuted("create-cluster") {
		t.Error("create-cluster step should have executed")
	}

	if rec.CallCount() != 1 {
		t.Errorf("expected step called once, got %d", rec.CallCount())
	}

	calls := rec.Calls()
	cfg := calls[0].Config
	if cfg["machine_type"] != "e2-standard-4" {
		t.Errorf("expected machine_type=e2-standard-4 in step config, got %v", cfg["machine_type"])
	}

	output := result.StepOutput("create-cluster")
	if output["status"] != "RUNNING" {
		t.Errorf("expected status=RUNNING, got %v", output["status"])
	}
	if output["cluster_name"] != "prod-cluster" {
		t.Errorf("expected cluster_name=prod-cluster, got %v", output["cluster_name"])
	}
}

// TestIntegration_GCPMultiStepPipeline tests a pipeline with multiple GCP resource steps in sequence.
func TestIntegration_GCPMultiStepPipeline(t *testing.T) {
	deployRec := wftest.RecordStep("step.gcp_deploy_service")
	deployRec.WithOutput(map[string]any{
		"service_url": "https://api-xyz.a.run.app",
		"status":      "deployed",
	})

	storageRec := wftest.RecordStep("step.gcp_create_bucket")
	storageRec.WithOutput(map[string]any{
		"bucket_name": "my-project-assets",
		"location":    "US",
		"status":      "created",
	})

	iamRec := wftest.RecordStep("step.gcp_grant_iam")
	iamRec.WithOutput(map[string]any{
		"role":   "roles/storage.objectViewer",
		"member": "serviceAccount:api-sa@my-project.iam.gserviceaccount.com",
		"status": "granted",
	})

	h := wftest.New(t,
		wftest.WithYAML(`
pipelines:
  gcp-full-deploy:
    steps:
      - name: deploy-service
        type: step.gcp_deploy_service
        config:
          resource_type: infra.container_service
          project: my-project
          region: us-central1
          image: gcr.io/my-project/api:v2
      - name: create-storage
        type: step.gcp_create_bucket
        config:
          resource_type: infra.storage
          project: my-project
          bucket: my-project-assets
          location: US
      - name: grant-iam
        type: step.gcp_grant_iam
        config:
          resource_type: infra.iam_role
          project: my-project
          role: roles/storage.objectViewer
          member: serviceAccount:api-sa@my-project.iam.gserviceaccount.com
`),
		deployRec,
		storageRec,
		iamRec,
	)

	result := h.ExecutePipeline("gcp-full-deploy", nil)
	if result.Error != nil {
		t.Fatalf("pipeline failed: %v", result.Error)
	}

	// All three steps must have executed.
	for _, stepName := range []string{"deploy-service", "create-storage", "grant-iam"} {
		if !result.StepExecuted(stepName) {
			t.Errorf("step %q should have executed", stepName)
		}
	}
	if result.StepCount() != 3 {
		t.Errorf("expected 3 steps executed, got %d", result.StepCount())
	}

	// Verify each mock was called exactly once.
	if deployRec.CallCount() != 1 {
		t.Errorf("deploy step: expected 1 call, got %d", deployRec.CallCount())
	}
	if storageRec.CallCount() != 1 {
		t.Errorf("storage step: expected 1 call, got %d", storageRec.CallCount())
	}
	if iamRec.CallCount() != 1 {
		t.Errorf("iam step: expected 1 call, got %d", iamRec.CallCount())
	}

	// Check per-step outputs.
	if out := result.StepOutput("deploy-service"); out["status"] != "deployed" {
		t.Errorf("deploy-service status: expected deployed, got %v", out["status"])
	}
	if out := result.StepOutput("create-storage"); out["bucket_name"] != "my-project-assets" {
		t.Errorf("create-storage bucket_name: expected my-project-assets, got %v", out["bucket_name"])
	}
	if out := result.StepOutput("grant-iam"); out["status"] != "granted" {
		t.Errorf("grant-iam status: expected granted, got %v", out["status"])
	}
}
