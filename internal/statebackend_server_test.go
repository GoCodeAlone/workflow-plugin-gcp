package internal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/statebackend"
	pb "github.com/GoCodeAlone/workflow/plugin/external/proto"
)

// Compile-time guard: gcpIaCServer MUST satisfy the typed state-backend
// contract so the SDK serve hook auto-registers it at plugin startup.
var _ pb.IaCStateBackendServer = (*gcpIaCServer)(nil)

func TestIaCServer_ListBackendNames(t *testing.T) {
	s := NewIaCServer()
	resp, err := s.ListBackendNames(context.Background(), &pb.ListBackendNamesRequest{})
	if err != nil {
		t.Fatalf("ListBackendNames: %v", err)
	}
	got := resp.GetBackendNames()
	if len(got) != 1 || got[0] != "gcs" {
		t.Errorf("ListBackendNames = %v, want [gcs]", got)
	}
}

func TestIaCServer_StateBackend_NotConfigured(t *testing.T) {
	s := NewIaCServer()
	// With no store injected, the state RPCs must return a clear error rather
	// than panicking on a nil store.
	if _, err := s.GetState(context.Background(), &pb.GetStateRequest{ResourceId: "x"}); err == nil {
		t.Error("GetState: expected error when backend not configured")
	}
	if _, err := s.SaveState(context.Background(), &pb.SaveStateRequest{State: &pb.IaCState{ResourceId: "x"}}); err == nil {
		t.Error("SaveState: expected error when backend not configured")
	}
	if _, err := s.ListStates(context.Background(), &pb.ListStatesRequest{}); err == nil {
		t.Error("ListStates: expected error when backend not configured")
	}
	if _, err := s.DeleteState(context.Background(), &pb.DeleteStateRequest{ResourceId: "x"}); err == nil {
		t.Error("DeleteState: expected error when backend not configured")
	}
	if _, err := s.Lock(context.Background(), &pb.LockRequest{ResourceId: "x"}); err == nil {
		t.Error("Lock: expected error when backend not configured")
	}
	if _, err := s.Unlock(context.Background(), &pb.UnlockRequest{ResourceId: "x"}); err == nil {
		t.Error("Unlock: expected error when backend not configured")
	}
}

func TestIaCServer_StateBackend_Configure(t *testing.T) {
	// The GCS client uses Application Default Credentials; point ADC at a
	// structurally-valid throwaway service-account key so cloud.google.com/go/
	// storage.NewClient constructs without network access.
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", writeFakeADC(t))

	s := NewIaCServer()

	// Before Configure, the backend is unconfigured — resolveStore must fail.
	if _, err := s.stateBackend.resolveStore(); err == nil {
		t.Fatal("resolveStore: expected FailedPrecondition before Configure")
	}

	cfg := map[string]any{
		"bucket": "tfstate",
		"prefix": "iac-state/",
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal cfg: %v", err)
	}
	if _, err := s.Configure(context.Background(), &pb.ConfigureRequest{
		BackendName: "gcs",
		ConfigJson:  cfgJSON,
	}); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// After Configure, the lazily-constructed store must resolve.
	store, err := s.stateBackend.resolveStore()
	if err != nil {
		t.Fatalf("resolveStore after Configure: %v", err)
	}
	if store == nil {
		t.Fatal("resolveStore after Configure: store is nil")
	}

	// A Configure for a backend name this plugin does not serve must be rejected.
	if _, err := s.Configure(context.Background(), &pb.ConfigureRequest{
		BackendName: "s3",
		ConfigJson:  cfgJSON,
	}); err == nil {
		t.Error("Configure: expected error for unknown backend name")
	}

	// A Configure missing the required 'bucket' config must be rejected.
	noBucket, _ := json.Marshal(map[string]any{"prefix": "iac-state/"})
	if _, err := s.Configure(context.Background(), &pb.ConfigureRequest{
		BackendName: "gcs",
		ConfigJson:  noBucket,
	}); err == nil {
		t.Error("Configure: expected error when 'bucket' config is missing")
	}
}

func TestIaCServer_StateBackend_RoundTrip(t *testing.T) {
	s := NewIaCServer()
	store := statebackend.NewGCSIaCStateStoreWithClient(newMockGCSStateClient(), "test-bucket", "iac-state/")
	s.stateBackend.setStateStore(store)

	ctx := context.Background()
	in := &pb.IaCState{
		ResourceId:   "gcs-rt",
		ResourceType: "kubernetes",
		Provider:     "gcp",
		Status:       "active",
		OutputsJson:  []byte(`{"endpoint":"https://gcs.example.com"}`),
		ConfigJson:   []byte(`{"region":"us-central1"}`),
	}
	if _, err := s.SaveState(ctx, &pb.SaveStateRequest{State: in}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	got, err := s.GetState(ctx, &pb.GetStateRequest{ResourceId: "gcs-rt"})
	if err != nil {
		t.Fatalf("GetState: %v", err)
	}
	if !got.GetExists() {
		t.Fatal("GetState: expected exists=true")
	}
	if got.GetState().GetProvider() != "gcp" {
		t.Errorf("Provider = %q, want gcp", got.GetState().GetProvider())
	}
	if string(got.GetState().GetOutputsJson()) != `{"endpoint":"https://gcs.example.com"}` {
		t.Errorf("OutputsJson round-trip mismatch: %s", got.GetState().GetOutputsJson())
	}

	list, err := s.ListStates(ctx, &pb.ListStatesRequest{})
	if err != nil {
		t.Fatalf("ListStates: %v", err)
	}
	if len(list.GetStates()) != 1 {
		t.Errorf("ListStates = %d, want 1", len(list.GetStates()))
	}

	if _, err := s.Lock(ctx, &pb.LockRequest{ResourceId: "gcs-rt"}); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if _, err := s.Unlock(ctx, &pb.UnlockRequest{ResourceId: "gcs-rt"}); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	if _, err := s.DeleteState(ctx, &pb.DeleteStateRequest{ResourceId: "gcs-rt"}); err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	after, err := s.GetState(ctx, &pb.GetStateRequest{ResourceId: "gcs-rt"})
	if err != nil {
		t.Fatalf("GetState after delete: %v", err)
	}
	if after.GetExists() {
		t.Error("GetState after delete: expected exists=false")
	}
}

// writeFakeADC writes a structurally-valid throwaway service-account JSON to a
// temp file and returns its path. The RSA key is freshly generated — no real
// credential — and storage.NewClient only parses it (no network) at construction.
func writeFakeADC(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	sa := map[string]string{
		"type":           "service_account",
		"project_id":     "test-project",
		"private_key_id": "test-key-id",
		"private_key":    string(keyPEM),
		"client_email":   "test@test-project.iam.gserviceaccount.com",
		"client_id":      "123456789",
		"token_uri":      "https://oauth2.googleapis.com/token",
	}
	b, err := json.Marshal(sa)
	if err != nil {
		t.Fatalf("marshal fake SA: %v", err)
	}
	path := filepath.Join(t.TempDir(), "fake-adc.json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatalf("write fake SA: %v", err)
	}
	return path
}

// mockGCSStateClient is an in-memory statebackend.GCSObjectClient for the
// round-trip test, with GCS generation-match lock semantics.
type mockGCSStateClient struct {
	mu         sync.Mutex
	objects    map[string][]byte
	generation map[string]int64
}

func newMockGCSStateClient() *mockGCSStateClient {
	return &mockGCSStateClient{
		objects:    make(map[string][]byte),
		generation: make(map[string]int64),
	}
}

func (m *mockGCSStateClient) ReadObject(_ context.Context, key string) ([]byte, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[key]
	if !ok {
		return nil, 0, statebackend.ErrGCSNotFound
	}
	return data, m.generation[key], nil
}

func (m *mockGCSStateClient) WriteObject(_ context.Context, key string, data []byte, _ string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.generation[key]++
	m.objects[key] = data
	return m.generation[key], nil
}

func (m *mockGCSStateClient) WriteObjectIfGenerationMatch(_ context.Context, key string, data []byte, _ string, generation int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	curr := m.generation[key]
	if generation == 0 {
		if _, exists := m.objects[key]; exists {
			return 0, fmt.Errorf("precondition failed: object exists")
		}
	} else if curr != generation {
		return 0, fmt.Errorf("precondition failed: generation mismatch (want %d, have %d)", generation, curr)
	}
	m.generation[key]++
	m.objects[key] = data
	return m.generation[key], nil
}

func (m *mockGCSStateClient) DeleteObject(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.objects[key]; !ok {
		return statebackend.ErrGCSNotFound
	}
	delete(m.objects, key)
	delete(m.generation, key)
	return nil
}

func (m *mockGCSStateClient) ListObjects(_ context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var keys []string
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}
