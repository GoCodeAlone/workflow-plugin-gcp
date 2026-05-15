// storage_gcs.go — plugin-native storage.gcs module.
//
// Ports workflow core's module/storage_gcs.go (GCSStorage) into the gcp
// plugin. Credentials flow through gcpcreds.BuildGCPOptions: either an
// inline `credentials:` block in the module config, or `credentials_ref:`
// resolving to a gcp.credentials module registered in the credref registry.
// Workflow core's `iac.state.gcs` and `storage.gcs` always used Application
// Default Credentials; this plugin-native form ADDS the inline / ref paths
// (DRY across multiple storage.gcs modules in one config) while preserving
// ADC as the empty-input fallback.
package modules

import (
	"context"
	"fmt"
	"io"
	"sync"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/credref"
	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/gcpcreds"
	sdk "github.com/GoCodeAlone/workflow/plugin/external/sdk"
	"github.com/GoCodeAlone/workflow/store"
)

// GCSStorageProvider implements sdk.ModuleProvider for the "storage.gcs"
// standalone-module type.
type GCSStorageProvider struct{}

// NewGCSStorageProvider returns a fresh provider.
func NewGCSStorageProvider() *GCSStorageProvider {
	return &GCSStorageProvider{}
}

// ModuleTypes reports the single module type this Provider serves.
func (p *GCSStorageProvider) ModuleTypes() []string {
	return []string{"storage.gcs"}
}

// CreateModule parses the storage.gcs config and returns a lifecycle-ready
// module instance. Bucket is required. Credentials come from either an
// inline `credentials:` sub-block OR `credentials_ref:` (a sibling
// gcp.credentials module name registered in the credref registry).
// `credentials:` and `credentials_ref:` are mutually exclusive; inline wins
// when both are supplied (to mirror upstream config-merge semantics).
func (p *GCSStorageProvider) CreateModule(_, name string, config map[string]any) (sdk.ModuleInstance, error) {
	bucket := stringField(config, "bucket")
	if bucket == "" {
		return nil, fmt.Errorf("storage.gcs %q: 'bucket' is required", name)
	}

	cred, err := resolveGCPCredentials(name, config)
	if err != nil {
		return nil, err
	}

	return &gcsStorageInstance{
		name:   name,
		bucket: bucket,
		prefix: stringField(config, "prefix"),
		cred:   cred,
	}, nil
}

// resolveGCPCredentials decodes the config's credentials surface into a
// gcpcreds.CredInput. An inline `credentials:` block beats `credentials_ref:`;
// a credentials_ref to an unregistered name is a clean error (no silent
// fallback to ADC — the user explicitly asked for that ref).
func resolveGCPCredentials(moduleName string, config map[string]any) (gcpcreds.CredInput, error) {
	var cred gcpcreds.CredInput
	if credsMap, ok := config["credentials"].(map[string]any); ok && len(credsMap) > 0 {
		cred.ProjectID = stringField(credsMap, "projectId")
		if sa := stringField(credsMap, "serviceAccountJson"); sa != "" {
			cred.ServiceAccountJSON = []byte(sa)
		}
		return cred, nil
	}
	if ref := stringField(config, "credentials_ref"); ref != "" {
		c, ok := credref.Resolve(ref)
		if !ok {
			return gcpcreds.CredInput{}, fmt.Errorf(
				"storage.gcs %q: credentials_ref %q not found; declare a gcp.credentials module first",
				moduleName, ref)
		}
		return c, nil
	}
	// No credentials surface → ADC fallback (BuildGCPOptions returns no
	// options, the SDK's default credential chain applies).
	return cred, nil
}

// gcsStorageInstance is the lifecycle + storage surface returned by
// CreateModule. The GCS client is constructed lazily in Start so a config
// can be loaded (and the module registered) without ADC being available at
// that moment — exactly mirroring workflow core's GCSStorage shape.
type gcsStorageInstance struct {
	name   string
	bucket string
	prefix string
	cred   gcpcreds.CredInput

	mu     sync.Mutex
	client *storage.Client
	// testBucket is a test-only injection seam — when set, getBucket()
	// returns it in place of the real *storage.BucketHandle wrapper.
	testBucket gcsBucketHandle
}

// SetTestBucketHandle injects a gcsBucketHandle for tests so Storage
// operations can be exercised without a real GCS endpoint.
func (m *gcsStorageInstance) SetTestBucketHandle(bh gcsBucketHandle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.testBucket = bh
}

func (m *gcsStorageInstance) Init() error { return nil }

func (m *gcsStorageInstance) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.testBucket != nil {
		return nil
	}
	opts := gcpcreds.BuildGCPOptions(m.cred)
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("storage.gcs %q: create client: %w", m.name, err)
	}
	m.client = client
	return nil
}

func (m *gcsStorageInstance) Stop(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil {
		if err := m.client.Close(); err != nil {
			return fmt.Errorf("storage.gcs %q: close client: %w", m.name, err)
		}
		m.client = nil
	}
	return nil
}

// getBucket returns the bucket handle, preferring the injected test handle.
func (m *gcsStorageInstance) getBucket() gcsBucketHandle {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.testBucket != nil {
		return m.testBucket
	}
	if m.client == nil {
		return nil
	}
	return &realBucketHandle{bh: m.client.Bucket(m.bucket)}
}

// List, Get, Put, Delete, Stat, MkdirAll mirror workflow core's
// module.GCSStorage public surface (module/storage_gcs.go).

func (m *gcsStorageInstance) List(ctx context.Context, prefix string) ([]store.FileInfo, error) {
	bh := m.getBucket()
	if bh == nil {
		return nil, fmt.Errorf("storage.gcs %q: client not initialized; call Start first", m.name)
	}
	it := bh.Objects(ctx, &storage.Query{Prefix: prefix})
	var files []store.FileInfo
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("storage.gcs %q: list %q: %w", m.name, prefix, err)
		}
		files = append(files, store.FileInfo{
			Name:    attrs.Name,
			Path:    attrs.Name,
			Size:    attrs.Size,
			ModTime: attrs.Updated,
		})
	}
	return files, nil
}

func (m *gcsStorageInstance) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	bh := m.getBucket()
	if bh == nil {
		return nil, fmt.Errorf("storage.gcs %q: client not initialized; call Start first", m.name)
	}
	r, err := bh.Object(key).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.gcs %q: get %q: %w", m.name, key, err)
	}
	return r, nil
}

func (m *gcsStorageInstance) Put(ctx context.Context, key string, reader io.Reader) error {
	bh := m.getBucket()
	if bh == nil {
		return fmt.Errorf("storage.gcs %q: client not initialized; call Start first", m.name)
	}
	w := bh.Object(key).NewWriter(ctx)
	if _, err := io.Copy(w, reader); err != nil {
		_ = w.Close()
		return fmt.Errorf("storage.gcs %q: put %q: %w", m.name, key, err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("storage.gcs %q: close writer for %q: %w", m.name, key, err)
	}
	return nil
}

func (m *gcsStorageInstance) Delete(ctx context.Context, key string) error {
	bh := m.getBucket()
	if bh == nil {
		return fmt.Errorf("storage.gcs %q: client not initialized; call Start first", m.name)
	}
	if err := bh.Object(key).Delete(ctx); err != nil {
		return fmt.Errorf("storage.gcs %q: delete %q: %w", m.name, key, err)
	}
	return nil
}

func (m *gcsStorageInstance) Stat(ctx context.Context, key string) (store.FileInfo, error) {
	bh := m.getBucket()
	if bh == nil {
		return store.FileInfo{}, fmt.Errorf("storage.gcs %q: client not initialized; call Start first", m.name)
	}
	attrs, err := bh.Object(key).Attrs(ctx)
	if err != nil {
		return store.FileInfo{}, fmt.Errorf("storage.gcs %q: stat %q: %w", m.name, key, err)
	}
	return store.FileInfo{
		Name:    attrs.Name,
		Path:    attrs.Name,
		Size:    attrs.Size,
		ModTime: attrs.Updated,
	}, nil
}

func (m *gcsStorageInstance) MkdirAll(_ context.Context, _ string) error {
	// Object storage has no real directories — no-op, matching upstream.
	return nil
}

// ── Test-seam interfaces (mirror workflow core's storage_gcs.go) ────────────

type gcsBucketHandle interface {
	Objects(ctx context.Context, q *storage.Query) objectIterator
	Object(name string) objectHandle
}

type objectIterator interface {
	Next() (*storage.ObjectAttrs, error)
}

type objectHandle interface {
	NewReader(ctx context.Context) (io.ReadCloser, error)
	NewWriter(ctx context.Context) io.WriteCloser
	Delete(ctx context.Context) error
	Attrs(ctx context.Context) (*storage.ObjectAttrs, error)
}

// realBucketHandle wraps *storage.BucketHandle to satisfy gcsBucketHandle.
type realBucketHandle struct{ bh *storage.BucketHandle }

func (r *realBucketHandle) Objects(ctx context.Context, q *storage.Query) objectIterator {
	return r.bh.Objects(ctx, q)
}

func (r *realBucketHandle) Object(name string) objectHandle {
	return &realObjectHandle{oh: r.bh.Object(name)}
}

type realObjectHandle struct{ oh *storage.ObjectHandle }

func (r *realObjectHandle) NewReader(ctx context.Context) (io.ReadCloser, error) {
	return r.oh.NewReader(ctx)
}

func (r *realObjectHandle) NewWriter(ctx context.Context) io.WriteCloser {
	return r.oh.NewWriter(ctx)
}

func (r *realObjectHandle) Delete(ctx context.Context) error { return r.oh.Delete(ctx) }

func (r *realObjectHandle) Attrs(ctx context.Context) (*storage.ObjectAttrs, error) {
	return r.oh.Attrs(ctx)
}
