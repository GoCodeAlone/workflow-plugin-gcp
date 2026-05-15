package modules

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/credref"
	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/gcpcreds"
)

func TestGCSStorageProvider_ModuleTypes(t *testing.T) {
	p := NewGCSStorageProvider()
	if got := p.ModuleTypes(); len(got) != 1 || got[0] != "storage.gcs" {
		t.Errorf("ModuleTypes = %v, want [storage.gcs]", got)
	}
}

func TestGCSStorageProvider_CreateModule_RequiresBucket(t *testing.T) {
	p := NewGCSStorageProvider()
	if _, err := p.CreateModule("storage.gcs", "nobucket", map[string]any{}); err == nil {
		t.Fatal("expected error when bucket is missing")
	}
}

func TestGCSStorageProvider_CreateModule_InlineCredentials(t *testing.T) {
	p := NewGCSStorageProvider()
	cfg := map[string]any{
		"bucket": "b1",
		"prefix": "data/",
		"credentials": map[string]any{
			"projectId":          "proj",
			"serviceAccountJson": `{"type":"service_account"}`,
		},
	}
	inst, err := p.CreateModule("storage.gcs", "inline", cfg)
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	m, ok := inst.(*gcsStorageInstance)
	if !ok {
		t.Fatalf("CreateModule returned %T, want *gcsStorageInstance", inst)
	}
	if m.bucket != "b1" || m.prefix != "data/" {
		t.Errorf("bucket/prefix = %q/%q, want b1/data/", m.bucket, m.prefix)
	}
	if m.cred.ProjectID != "proj" {
		t.Errorf("ProjectID = %q, want proj", m.cred.ProjectID)
	}
	if string(m.cred.ServiceAccountJSON) != `{"type":"service_account"}` {
		t.Errorf("ServiceAccountJSON not preserved: %s", m.cred.ServiceAccountJSON)
	}
}

func TestGCSStorageProvider_CreateModule_CredentialsRef(t *testing.T) {
	t.Cleanup(credref.Reset)
	want := gcpcreds.CredInput{
		ProjectID:          "ref-proj",
		ServiceAccountJSON: []byte(`{"type":"service_account","project_id":"ref-proj"}`),
	}
	if err := credref.Register("shared-creds", want); err != nil {
		t.Fatalf("Register: %v", err)
	}

	p := NewGCSStorageProvider()
	cfg := map[string]any{
		"bucket":          "b2",
		"credentials_ref": "shared-creds",
	}
	inst, err := p.CreateModule("storage.gcs", "ref", cfg)
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	m := inst.(*gcsStorageInstance)
	if m.cred.ProjectID != want.ProjectID || string(m.cred.ServiceAccountJSON) != string(want.ServiceAccountJSON) {
		t.Errorf("resolved cred = %+v, want %+v", m.cred, want)
	}
}

func TestGCSStorageProvider_CreateModule_CredentialsRef_MissingErrors(t *testing.T) {
	t.Cleanup(credref.Reset)
	p := NewGCSStorageProvider()
	cfg := map[string]any{
		"bucket":          "b3",
		"credentials_ref": "does-not-exist",
	}
	_, err := p.CreateModule("storage.gcs", "missing", cfg)
	if err == nil {
		t.Fatal("expected error when credentials_ref is unregistered")
	}
	if !strings.Contains(err.Error(), "credentials_ref \"does-not-exist\" not found") {
		t.Errorf("error = %q, expected to mention the missing ref name", err)
	}
}

func TestGCSStorageProvider_CreateModule_InlineBeatsRef(t *testing.T) {
	t.Cleanup(credref.Reset)
	_ = credref.Register("would-lose", gcpcreds.CredInput{ProjectID: "ref-side"})
	p := NewGCSStorageProvider()
	cfg := map[string]any{
		"bucket": "b",
		"credentials": map[string]any{
			"projectId": "inline-side",
		},
		"credentials_ref": "would-lose",
	}
	inst, err := p.CreateModule("storage.gcs", "both", cfg)
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	m := inst.(*gcsStorageInstance)
	if m.cred.ProjectID != "inline-side" {
		t.Errorf("ProjectID = %q, want inline-side (inline credentials must beat credentials_ref)", m.cred.ProjectID)
	}
}

func TestGCSStorageProvider_CreateModule_NoCredsADCFallback(t *testing.T) {
	p := NewGCSStorageProvider()
	inst, err := p.CreateModule("storage.gcs", "adc", map[string]any{"bucket": "b"})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	m := inst.(*gcsStorageInstance)
	if m.cred.ProjectID != "" || m.cred.ServiceAccountJSON != nil {
		t.Errorf("expected zero CredInput (ADC fallback), got %+v", m.cred)
	}
}

// ── Lifecycle + Storage-operation tests (via test-seam mock bucket) ─────────

func TestGCSStorageInstance_Lifecycle_TestSeam(t *testing.T) {
	p := NewGCSStorageProvider()
	inst, err := p.CreateModule("storage.gcs", "lifecycle", map[string]any{"bucket": "b"})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	m := inst.(*gcsStorageInstance)
	m.SetTestBucketHandle(newMockBucket())

	if err := m.Init(); err != nil {
		t.Errorf("Init: %v", err)
	}
	// Start with the test seam set must skip storage.NewClient entirely.
	if err := m.Start(context.Background()); err != nil {
		t.Errorf("Start: %v", err)
	}
	if err := m.Stop(context.Background()); err != nil {
		t.Errorf("Stop: %v", err)
	}
}

func TestGCSStorageInstance_StorageOperations_RoundTrip(t *testing.T) {
	p := NewGCSStorageProvider()
	inst, _ := p.CreateModule("storage.gcs", "ops", map[string]any{"bucket": "b"})
	m := inst.(*gcsStorageInstance)
	mock := newMockBucket()
	m.SetTestBucketHandle(mock)
	_ = m.Start(context.Background())
	ctx := context.Background()

	// Put.
	if err := m.Put(ctx, "k1", bytes.NewReader([]byte("hello"))); err != nil {
		t.Fatalf("Put: %v", err)
	}
	// Get.
	r, err := m.Get(ctx, "k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, _ := io.ReadAll(r)
	_ = r.Close()
	if string(got) != "hello" {
		t.Errorf("Get returned %q, want hello", got)
	}
	// Stat.
	info, err := m.Stat(ctx, "k1")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Name != "k1" || info.Size != 5 {
		t.Errorf("Stat = %+v, want Name=k1 Size=5", info)
	}
	// List.
	files, err := m.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 1 || files[0].Name != "k1" {
		t.Errorf("List = %+v, want one k1", files)
	}
	// Delete.
	if err := m.Delete(ctx, "k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := m.Stat(ctx, "k1"); err == nil {
		t.Error("Stat after Delete: expected error")
	}
}

// mockBucket is an in-memory gcsBucketHandle for tests.
type mockBucket struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMockBucket() *mockBucket {
	return &mockBucket{objects: make(map[string][]byte)}
}

func (b *mockBucket) Objects(_ context.Context, q *storage.Query) objectIterator {
	b.mu.Lock()
	defer b.mu.Unlock()
	var attrs []*storage.ObjectAttrs
	prefix := ""
	if q != nil {
		prefix = q.Prefix
	}
	for name, data := range b.objects {
		if strings.HasPrefix(name, prefix) {
			attrs = append(attrs, &storage.ObjectAttrs{
				Name:    name,
				Size:    int64(len(data)),
				Updated: time.Unix(0, 0),
			})
		}
	}
	return &mockIterator{attrs: attrs}
}

func (b *mockBucket) Object(name string) objectHandle {
	return &mockObject{bucket: b, name: name}
}

type mockIterator struct {
	attrs []*storage.ObjectAttrs
	idx   int
}

func (it *mockIterator) Next() (*storage.ObjectAttrs, error) {
	if it.idx >= len(it.attrs) {
		return nil, iterator.Done
	}
	a := it.attrs[it.idx]
	it.idx++
	return a, nil
}

type mockObject struct {
	bucket *mockBucket
	name   string
}

func (o *mockObject) NewReader(_ context.Context) (io.ReadCloser, error) {
	o.bucket.mu.Lock()
	defer o.bucket.mu.Unlock()
	data, ok := o.bucket.objects[o.name]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (o *mockObject) NewWriter(_ context.Context) io.WriteCloser {
	return &mockWriter{bucket: o.bucket, name: o.name, buf: &bytes.Buffer{}}
}

func (o *mockObject) Delete(_ context.Context) error {
	o.bucket.mu.Lock()
	defer o.bucket.mu.Unlock()
	if _, ok := o.bucket.objects[o.name]; !ok {
		return errors.New("not found")
	}
	delete(o.bucket.objects, o.name)
	return nil
}

func (o *mockObject) Attrs(_ context.Context) (*storage.ObjectAttrs, error) {
	o.bucket.mu.Lock()
	defer o.bucket.mu.Unlock()
	data, ok := o.bucket.objects[o.name]
	if !ok {
		return nil, errors.New("not found")
	}
	return &storage.ObjectAttrs{
		Name:    o.name,
		Size:    int64(len(data)),
		Updated: time.Unix(0, 0),
	}, nil
}

type mockWriter struct {
	bucket *mockBucket
	name   string
	buf    *bytes.Buffer
}

func (w *mockWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }

func (w *mockWriter) Close() error {
	w.bucket.mu.Lock()
	defer w.bucket.mu.Unlock()
	w.bucket.objects[w.name] = w.buf.Bytes()
	return nil
}
