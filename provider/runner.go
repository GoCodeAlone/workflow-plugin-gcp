package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	run "cloud.google.com/go/run/apiv2"
	runpb "cloud.google.com/go/run/apiv2/runpb"
	"github.com/GoCodeAlone/workflow/interfaces"
	"google.golang.org/api/option"
)

type gcpRunnerClient interface {
	CreateJob(ctx context.Context, parent, jobID string, job *runpb.Job) (*runpb.Job, error)
	RunJob(ctx context.Context, name string) (*runpb.Execution, error)
	GetExecution(ctx context.Context, name string) (*runpb.Execution, error)
}

type realGCPRunnerClient struct {
	jobs       *run.JobsClient
	executions *run.ExecutionsClient
}

func newRealGCPRunnerClient(ctx context.Context, opts ...option.ClientOption) (gcpRunnerClient, error) {
	jobs, err := run.NewJobsClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("cloud run jobs client: %w", err)
	}
	executions, err := run.NewExecutionsClient(ctx, opts...)
	if err != nil {
		_ = jobs.Close()
		return nil, fmt.Errorf("cloud run executions client: %w", err)
	}
	return &realGCPRunnerClient{jobs: jobs, executions: executions}, nil
}

func (c *realGCPRunnerClient) CreateJob(ctx context.Context, parent, jobID string, job *runpb.Job) (*runpb.Job, error) {
	op, err := c.jobs.CreateJob(ctx, &runpb.CreateJobRequest{
		Parent: parent,
		JobId:  jobID,
		Job:    job,
	})
	if err != nil {
		return nil, err
	}
	return op.Wait(ctx)
}

func (c *realGCPRunnerClient) RunJob(ctx context.Context, name string) (*runpb.Execution, error) {
	op, err := c.jobs.RunJob(ctx, &runpb.RunJobRequest{Name: name})
	if err != nil {
		return nil, err
	}
	return op.Wait(ctx)
}

func (c *realGCPRunnerClient) GetExecution(ctx context.Context, name string) (*runpb.Execution, error) {
	return c.executions.GetExecution(ctx, &runpb.GetExecutionRequest{Name: name})
}

var _ interfaces.IaCProviderRunner = (*GCPProvider)(nil)

func (p *GCPProvider) RunJob(ctx context.Context, spec interfaces.JobSpec) (*interfaces.JobHandle, error) {
	if p.runnerClient == nil || p.projectID == "" || p.region == "" {
		return nil, fmt.Errorf("gcp runner: provider is not initialized")
	}
	if strings.TrimSpace(spec.Image) == "" {
		return nil, fmt.Errorf("gcp runner: image is required")
	}
	if strings.TrimSpace(spec.RunCommand) == "" {
		return nil, fmt.Errorf("gcp runner: run_command is required")
	}

	jobID := gcpJobName(spec.Name)
	parent := fmt.Sprintf("projects/%s/locations/%s", p.projectID, p.region)
	job := &runpb.Job{
		Template: &runpb.ExecutionTemplate{
			Template: &runpb.TaskTemplate{
				Containers: []*runpb.Container{{
					Name:    jobID,
					Image:   spec.Image,
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", spec.RunCommand},
					Env:     gcpJobEnvironment(spec),
				}},
				Retries: &runpb.TaskTemplate_MaxRetries{MaxRetries: 0},
			},
		},
	}
	created, err := p.runnerClient.CreateJob(ctx, parent, jobID, job)
	if err != nil {
		return nil, fmt.Errorf("gcp runner: create Cloud Run job %q: %w", jobID, err)
	}
	jobName := created.GetName()
	if jobName == "" {
		jobName = parent + "/jobs/" + jobID
	}
	execution, err := p.runnerClient.RunJob(ctx, jobName)
	if err != nil {
		return nil, fmt.Errorf("gcp runner: run Cloud Run job %q: %w", jobName, err)
	}
	execName := execution.GetName()
	if execName == "" && created.GetLatestCreatedExecution() != nil {
		execName = created.GetLatestCreatedExecution().GetName()
	}
	if execName == "" {
		return nil, fmt.Errorf("gcp runner: Cloud Run job %q did not return an execution name", jobName)
	}
	return &interfaces.JobHandle{
		ID:       execName,
		Name:     jobID,
		Provider: "gcp",
		Metadata: map[string]string{
			"project_id": p.projectID,
			"region":     p.region,
			"job":        jobName,
			"execution":  execName,
		},
	}, nil
}

func (p *GCPProvider) JobStatus(ctx context.Context, handle interfaces.JobHandle) (*interfaces.JobStatusReply, error) {
	if p.runnerClient == nil {
		return nil, fmt.Errorf("gcp runner: provider is not initialized")
	}
	executionName := handle.Metadata["execution"]
	if executionName == "" {
		executionName = handle.ID
	}
	if executionName == "" {
		return nil, fmt.Errorf("gcp runner: execution metadata is required")
	}
	execution, err := p.runnerClient.GetExecution(ctx, executionName)
	if err != nil {
		return nil, fmt.Errorf("gcp runner: get execution %q: %w", executionName, err)
	}
	state, exitCode, message := gcpExecutionState(execution)
	return &interfaces.JobStatusReply{Handle: handle, State: state, ExitCode: exitCode, Message: message}, nil
}

func (p *GCPProvider) JobLogs(_ context.Context, _ interfaces.JobHandle, sink interfaces.LogCaptureSink) error {
	if p.runnerClient == nil {
		return fmt.Errorf("gcp runner: provider is not initialized")
	}
	if sink == nil {
		return nil
	}
	return sink.WriteLogChunk(interfaces.LogChunk{EOF: true})
}

func gcpJobEnvironment(spec interfaces.JobSpec) []*runpb.EnvVar {
	var out []*runpb.EnvVar
	for _, key := range sortedGCPMapKeys(spec.EnvVars) {
		out = append(out, &runpb.EnvVar{Name: key, Values: &runpb.EnvVar_Value{Value: spec.EnvVars[key]}})
	}
	for _, key := range sortedGCPMapKeys(spec.EnvVarsSecret) {
		secret, version := gcpSecretRef(spec.EnvVarsSecret[key])
		out = append(out, &runpb.EnvVar{
			Name: key,
			Values: &runpb.EnvVar_ValueSource{
				ValueSource: &runpb.EnvVarSource{
					SecretKeyRef: &runpb.SecretKeySelector{Secret: secret, Version: version},
				},
			},
		})
	}
	return out
}

func sortedGCPMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func gcpSecretRef(ref string) (string, string) {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "secret://")
	version := "latest"
	if i := strings.LastIndex(ref, ":"); i > 0 && i < len(ref)-1 {
		version = ref[i+1:]
		ref = ref[:i]
	}
	return ref, version
}

var nonGCPJobName = regexp.MustCompile(`[^a-z0-9-]+`)
var repeatedGCPHyphen = regexp.MustCompile(`-+`)

func gcpJobName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = nonGCPJobName.ReplaceAllString(name, "-")
	name = repeatedGCPHyphen.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "provider-ephemeral-job"
	}
	if name[0] < 'a' || name[0] > 'z' {
		name = "job-" + name
	}
	suffix := fmt.Sprintf("-%d", time.Now().UnixNano())
	maxBase := 63 - len(suffix)
	if len(name) > maxBase {
		name = strings.TrimRight(name[:maxBase], "-")
	}
	return name + suffix
}

func gcpExecutionState(execution *runpb.Execution) (interfaces.JobState, int, string) {
	if execution == nil {
		return interfaces.JobStateUnknown, 0, ""
	}
	if execution.GetFailedCount() > 0 {
		return interfaces.JobStateFailed, 1, gcpExecutionMessage(execution)
	}
	if execution.GetCompletionTime() != nil && execution.GetSucceededCount() >= execution.GetTaskCount() {
		return interfaces.JobStateSucceeded, 0, gcpExecutionMessage(execution)
	}
	if execution.GetRunningCount() > 0 || execution.GetReconciling() {
		return interfaces.JobStateRunning, 0, gcpExecutionMessage(execution)
	}
	return interfaces.JobStatePending, 0, gcpExecutionMessage(execution)
}

func gcpExecutionMessage(execution *runpb.Execution) string {
	for _, condition := range execution.GetConditions() {
		if condition.GetMessage() != "" {
			return condition.GetMessage()
		}
	}
	return ""
}
