package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	pb "github.com/GoCodeAlone/workflow/plugin/external/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	gcpCollectorModuleName = "observability-collector"
	gcpCollectorImage      = "otel/opentelemetry-collector-contrib:latest"
	gcpCollectorType       = "infra.container_service"
)

// MapRequirements maps canonical derived-IaC requirements into GCP-owned
// resource shapes. The v1 mapper emits a Cloud Run collector service. If an
// application needs a collector sidecar in its own Cloud Run service, it can
// supply an explicit module with the same satisfies keys.
func (s *gcpIaCServer) MapRequirements(_ context.Context, req *pb.MapRequirementsRequest) (*pb.MapRequirementsResponse, error) {
	if req.GetProvider() != "" && req.GetProvider() != "gcp" {
		return nil, status.Errorf(codes.InvalidArgument, "gcp mapper cannot satisfy provider %q", req.GetProvider())
	}

	resp := &pb.MapRequirementsResponse{}
	var accepted []*pb.IaCRequirement
	for _, requirement := range req.GetRequirements() {
		switch diag := gcpRejectUnsupportedRequirement(req.GetRuntime(), requirement); {
		case diag != nil:
			resp.Rejected = append(resp.Rejected, diag)
		default:
			accepted = append(accepted, requirement)
			resp.AcceptedKeys = append(resp.AcceptedKeys, requirement.GetKey())
		}
	}
	if len(accepted) == 0 {
		return resp, nil
	}

	configJSON, err := json.Marshal(gcpCollectorModuleConfig(accepted))
	if err != nil {
		return nil, fmt.Errorf("gcp requirement mapper: encode collector config: %w", err)
	}
	resp.Modules = append(resp.Modules, &pb.DerivedModuleSpec{
		Name:       gcpCollectorModuleName,
		Type:       gcpCollectorType,
		Satisfies:  append([]string(nil), resp.GetAcceptedKeys()...),
		ConfigJson: configJSON,
	})
	resp.Notes = append(resp.Notes, &pb.RequirementNote{
		Key:         strings.Join(resp.GetAcceptedKeys(), ","),
		Message:     "GCP Cloud Run derivation emits a generic OTel Collector service. Use an explicit infra.container_service module with the same satisfies keys when an application needs Cloud Run sidecars.",
		Interactive: false,
	})
	return resp, nil
}

func gcpRejectUnsupportedRequirement(runtime pb.RequirementRuntime, req *pb.IaCRequirement) *pb.RequirementDiagnostic {
	key := req.GetKey()
	if req.GetKind() != pb.RequirementKind_REQUIREMENT_KIND_OBSERVABILITY {
		return gcpRequirementDiagnostic(key, "unsupported_kind", "gcp can only derive observability requirements today")
	}
	if hint := req.GetResourceTypeHint(); hint != "" && hint != gcpCollectorType {
		return gcpRequirementDiagnostic(key, "unsupported_resource_type_hint",
			fmt.Sprintf("gcp observability derivation emits %s, not %s", gcpCollectorType, hint))
	}
	if runtime != pb.RequirementRuntime_REQUIREMENT_RUNTIME_CLOUD_RUN {
		return gcpRequirementDiagnostic(key, "unsupported_runtime", "gcp observability derivation currently targets Cloud Run")
	}
	if !gcpRequirementAllowsRuntime(req, runtime) {
		return gcpRequirementDiagnostic(key, "unsupported_runtime", "requirement does not allow Cloud Run")
	}
	if !gcpRequirementAllowsDeploymentMode(req) {
		return gcpRequirementDiagnostic(key, "unsupported_deployment_mode",
			"gcp Cloud Run maps sidecar intent to an explicit or sibling collector service; daemonset mode belongs to GKE and is not emitted by this mapper yet")
	}
	return nil
}

func gcpRequirementAllowsRuntime(req *pb.IaCRequirement, runtime pb.RequirementRuntime) bool {
	if len(req.GetRuntimes()) == 0 {
		return true
	}
	for _, candidate := range req.GetRuntimes() {
		if candidate == runtime {
			return true
		}
	}
	return false
}

func gcpRequirementAllowsDeploymentMode(req *pb.IaCRequirement) bool {
	modes := req.GetDeploymentModes()
	if len(modes) == 0 {
		return true
	}
	for _, mode := range modes {
		switch mode {
		case pb.DeploymentMode_DEPLOYMENT_MODE_SIDECAR,
			pb.DeploymentMode_DEPLOYMENT_MODE_SIBLING_SERVICE,
			pb.DeploymentMode_DEPLOYMENT_MODE_MANAGED:
			return true
		}
	}
	return false
}

func gcpRequirementDiagnostic(key, code, message string) *pb.RequirementDiagnostic {
	return &pb.RequirementDiagnostic{Key: key, Code: code, Message: message}
}

func gcpCollectorModuleConfig(reqs []*pb.IaCRequirement) map[string]any {
	signals := gcpRequestedSignals(reqs)
	backends := gcpRequestedBackends(reqs)
	collectorConfig := gcpBuildCollectorConfig(signals, backends)

	envVars := map[string]any{
		"OTELCOL_CONFIG": collectorConfig,
	}
	secretVars := make(map[string]any)
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_OTEL) {
		envVars["OTEL_EXPORTER_OTLP_ENDPOINT"] = "${vars.otel_exporter_otlp_endpoint}"
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_DATADOG) {
		envVars["DD_SITE"] = "${vars.datadog_site}"
		secretVars["DD_API_KEY"] = "${secrets.datadog_api_key}"
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_LOKI) {
		envVars["LOKI_ENDPOINT"] = "${vars.loki_endpoint}"
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_GRAFANA) {
		envVars["GRAFANA_OTLP_ENDPOINT"] = "${vars.grafana_otlp_endpoint}"
		secretVars["GRAFANA_OTLP_HEADERS"] = "${secrets.grafana_otlp_headers}"
	}

	return map[string]any{
		"image":           gcpCollectorImage,
		"command":         []any{"otelcol-contrib", "--config=env:OTELCOL_CONFIG"},
		"replicas":        1,
		"ports":           gcpCollectorPorts(backends),
		"env_vars":        envVars,
		"env_vars_secret": secretVars,
	}
}

func gcpRequestedSignals(reqs []*pb.IaCRequirement) map[pb.TelemetrySignal]bool {
	out := make(map[pb.TelemetrySignal]bool)
	for _, req := range reqs {
		for _, signal := range req.GetTelemetrySignals() {
			if signal != pb.TelemetrySignal_TELEMETRY_SIGNAL_UNSPECIFIED {
				out[signal] = true
			}
		}
	}
	if len(out) == 0 {
		out[pb.TelemetrySignal_TELEMETRY_SIGNAL_TRACES] = true
		out[pb.TelemetrySignal_TELEMETRY_SIGNAL_METRICS] = true
		out[pb.TelemetrySignal_TELEMETRY_SIGNAL_LOGS] = true
	}
	return out
}

func gcpRequestedBackends(reqs []*pb.IaCRequirement) map[pb.ObservabilityBackend]bool {
	out := make(map[pb.ObservabilityBackend]bool)
	for _, req := range reqs {
		for _, backend := range req.GetObservabilityBackends() {
			if backend != pb.ObservabilityBackend_OBSERVABILITY_BACKEND_UNSPECIFIED {
				out[backend] = true
			}
		}
	}
	if len(out) == 0 {
		out[pb.ObservabilityBackend_OBSERVABILITY_BACKEND_OTEL] = true
	}
	return out
}

func gcpCollectorPorts(backends map[pb.ObservabilityBackend]bool) []any {
	ports := []any{
		map[string]any{"port": 4317, "public": false},
		map[string]any{"port": 4318, "public": false},
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_PROMETHEUS) {
		ports = append(ports, map[string]any{"port": 9464, "public": false})
	}
	return ports
}

func gcpBuildCollectorConfig(signals map[pb.TelemetrySignal]bool, backends map[pb.ObservabilityBackend]bool) string {
	var b strings.Builder
	b.WriteString("receivers:\n")
	b.WriteString("  otlp:\n")
	b.WriteString("    protocols:\n")
	b.WriteString("      grpc:\n")
	b.WriteString("        endpoint: 0.0.0.0:4317\n")
	b.WriteString("      http:\n")
	b.WriteString("        endpoint: 0.0.0.0:4318\n")
	b.WriteString("processors:\n")
	b.WriteString("  batch: {}\n")
	b.WriteString("exporters:\n")
	gcpWriteExporters(&b, backends)
	b.WriteString("service:\n")
	b.WriteString("  pipelines:\n")
	if signals[pb.TelemetrySignal_TELEMETRY_SIGNAL_TRACES] {
		gcpWritePipeline(&b, "traces", gcpExportersForSignal(pb.TelemetrySignal_TELEMETRY_SIGNAL_TRACES, backends))
	}
	if signals[pb.TelemetrySignal_TELEMETRY_SIGNAL_METRICS] {
		gcpWritePipeline(&b, "metrics", gcpExportersForSignal(pb.TelemetrySignal_TELEMETRY_SIGNAL_METRICS, backends))
	}
	if signals[pb.TelemetrySignal_TELEMETRY_SIGNAL_LOGS] {
		gcpWritePipeline(&b, "logs", gcpExportersForSignal(pb.TelemetrySignal_TELEMETRY_SIGNAL_LOGS, backends))
	}
	return b.String()
}

func gcpWriteExporters(b *strings.Builder, backends map[pb.ObservabilityBackend]bool) {
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_OTEL) {
		b.WriteString("  otlp:\n")
		b.WriteString("    endpoint: ${env:OTEL_EXPORTER_OTLP_ENDPOINT}\n")
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_DATADOG) {
		b.WriteString("  datadog:\n")
		b.WriteString("    api:\n")
		b.WriteString("      key: ${env:DD_API_KEY}\n")
		b.WriteString("      site: ${env:DD_SITE}\n")
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_PROMETHEUS) {
		b.WriteString("  prometheus:\n")
		b.WriteString("    endpoint: 0.0.0.0:9464\n")
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_LOKI) {
		b.WriteString("  loki:\n")
		b.WriteString("    endpoint: ${env:LOKI_ENDPOINT}\n")
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_GRAFANA) {
		b.WriteString("  otlp/grafana_otlp:\n")
		b.WriteString("    endpoint: ${env:GRAFANA_OTLP_ENDPOINT}\n")
		b.WriteString("    headers:\n")
		b.WriteString("      authorization: ${env:GRAFANA_OTLP_HEADERS}\n")
	}
}

func gcpWritePipeline(b *strings.Builder, name string, exporters []string) {
	if len(exporters) == 0 {
		return
	}
	b.WriteString("    ")
	b.WriteString(name)
	b.WriteString(":\n")
	b.WriteString("      receivers: [otlp]\n")
	b.WriteString("      processors: [batch]\n")
	b.WriteString("      exporters: [")
	b.WriteString(strings.Join(exporters, ", "))
	b.WriteString("]\n")
}

func gcpExportersForSignal(signal pb.TelemetrySignal, backends map[pb.ObservabilityBackend]bool) []string {
	var exporters []string
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_OTEL) {
		exporters = append(exporters, "otlp")
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_DATADOG) {
		exporters = append(exporters, "datadog")
	}
	if signal == pb.TelemetrySignal_TELEMETRY_SIGNAL_METRICS &&
		gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_PROMETHEUS) {
		exporters = append(exporters, "prometheus")
	}
	if signal == pb.TelemetrySignal_TELEMETRY_SIGNAL_LOGS &&
		gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_LOKI) {
		exporters = append(exporters, "loki")
	}
	if gcpHasBackend(backends, pb.ObservabilityBackend_OBSERVABILITY_BACKEND_GRAFANA) {
		exporters = append(exporters, "otlp/grafana_otlp")
	}
	sort.Strings(exporters)
	return exporters
}

func gcpHasBackend(backends map[pb.ObservabilityBackend]bool, backend pb.ObservabilityBackend) bool {
	return backends[backend]
}
