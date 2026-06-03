package provider

import (
	"context"
	"strings"
	"testing"

	runpb "cloud.google.com/go/run/apiv2/runpb"
	"github.com/GoCodeAlone/workflow/interfaces"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeGCPRunnerClient struct {
	parent       string
	jobID        string
	job          *runpb.Job
	execution    *runpb.Execution
	runExecution *runpb.Execution
	gotExecution string
}

func (f *fakeGCPRunnerClient) CreateJob(_ context.Context, parent, jobID string, job *runpb.Job) (*runpb.Job, error) {
	f.parent = parent
	f.jobID = jobID
	f.job = job
	return &runpb.Job{Name: parent + "/jobs/" + jobID}, nil
}

func (f *fakeGCPRunnerClient) RunJob(_ context.Context, name string) (*runpb.Execution, error) {
	if f.runExecution != nil {
		return f.runExecution, nil
	}
	return &runpb.Execution{Name: name + "/executions/run-1", RunningCount: 1, TaskCount: 1}, nil
}

func (f *fakeGCPRunnerClient) GetExecution(_ context.Context, name string) (*runpb.Execution, error) {
	f.gotExecution = name
	return f.execution, nil
}

func TestGCPRunnerRunJobCreatesCloudRunJob(t *testing.T) {
	client := &fakeGCPRunnerClient{}
	p := &GCPProvider{projectID: "proj", region: "us-central1", runnerClient: client}

	handle, err := p.RunJob(context.Background(), interfaces.JobSpec{
		Name:          "Migrate DB!",
		Image:         "us-docker.pkg.dev/proj/app/migrate:latest",
		RunCommand:    "bin/migrate up",
		EnvVars:       map[string]string{"PLAIN": "value"},
		EnvVarsSecret: map[string]string{"DATABASE_URL": "database-url:3"},
	})
	if err != nil {
		t.Fatalf("RunJob returned error: %v", err)
	}
	if handle.Provider != "gcp" || handle.Metadata["execution"] == "" {
		t.Fatalf("handle = %+v", handle)
	}
	if client.parent != "projects/proj/locations/us-central1" {
		t.Fatalf("parent = %q", client.parent)
	}
	if !strings.HasPrefix(client.jobID, "migrate-db-") {
		t.Fatalf("jobID = %q", client.jobID)
	}
	container := client.job.Template.Template.Containers[0]
	if container.Image != "us-docker.pkg.dev/proj/app/migrate:latest" {
		t.Fatalf("image = %q", container.Image)
	}
	if len(container.Command) != 1 || container.Command[0] != "/bin/sh" ||
		len(container.Args) != 2 || container.Args[0] != "-c" || container.Args[1] != "bin/migrate up" {
		t.Fatalf("command=%v args=%v", container.Command, container.Args)
	}
	if !hasGCPEnv(container.Env, "PLAIN", "value") {
		t.Fatalf("missing plain env: %#v", container.Env)
	}
	if !hasGCPSecretEnv(container.Env, "DATABASE_URL", "database-url", "3") {
		t.Fatalf("missing secret env: %#v", container.Env)
	}
}

func TestGCPRunnerStatusAndLogs(t *testing.T) {
	client := &fakeGCPRunnerClient{execution: &runpb.Execution{
		Name:           "projects/proj/locations/us-central1/jobs/job/executions/run-1",
		TaskCount:      1,
		SucceededCount: 1,
		CompletionTime: timestamppb.Now(),
	}}
	p := &GCPProvider{runnerClient: client}
	handle := interfaces.JobHandle{ID: client.execution.Name, Metadata: map[string]string{"execution": client.execution.Name}}

	status, err := p.JobStatus(context.Background(), handle)
	if err != nil {
		t.Fatalf("JobStatus returned error: %v", err)
	}
	if status.State != interfaces.JobStateSucceeded || status.ExitCode != 0 {
		t.Fatalf("status = %+v", status)
	}

	sink := &runnerSink{}
	if err := p.JobLogs(context.Background(), handle, sink); err != nil {
		t.Fatalf("JobLogs returned error: %v", err)
	}
	if !sink.eof {
		t.Fatal("JobLogs did not send EOF")
	}
}

func TestGCPRunnerRunJobRequiresExecutionName(t *testing.T) {
	client := &fakeGCPRunnerClient{runExecution: &runpb.Execution{}}
	p := &GCPProvider{projectID: "proj", region: "us-central1", runnerClient: client}
	_, err := p.RunJob(context.Background(), interfaces.JobSpec{
		Name:       "job",
		Image:      "repo/app:latest",
		RunCommand: "echo ok",
	})
	if err == nil || !strings.Contains(err.Error(), "did not return an execution name") {
		t.Fatalf("RunJob error = %v", err)
	}
}

func TestGCPJobNameStartsWithLetterAndCollapsesHyphens(t *testing.T) {
	name := gcpJobName("123--bad name")
	if !strings.HasPrefix(name, "job-123-bad-name-") {
		t.Fatalf("name = %q", name)
	}
	if strings.Contains(name, "--") {
		t.Fatalf("name contains repeated hyphen: %q", name)
	}
}

func hasGCPEnv(values []*runpb.EnvVar, key, value string) bool {
	for _, env := range values {
		if env.GetName() == key && env.GetValue() == value {
			return true
		}
	}
	return false
}

func hasGCPSecretEnv(values []*runpb.EnvVar, key, secret, version string) bool {
	for _, env := range values {
		if env.GetName() != key || env.GetValueSource() == nil || env.GetValueSource().GetSecretKeyRef() == nil {
			continue
		}
		ref := env.GetValueSource().GetSecretKeyRef()
		return ref.GetSecret() == secret && ref.GetVersion() == version
	}
	return false
}

type runnerSink struct {
	eof bool
}

func (s *runnerSink) WriteLogChunk(chunk interfaces.LogChunk) error {
	if chunk.EOF {
		s.eof = true
	}
	return nil
}
