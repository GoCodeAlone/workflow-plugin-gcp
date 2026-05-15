package credref

import (
	"sync"
	"testing"

	"github.com/GoCodeAlone/workflow-plugin-gcp/internal/gcpcreds"
)

func TestRegister_FirstCallSucceeds(t *testing.T) {
	t.Cleanup(Reset)
	if err := Register("primary", gcpcreds.CredInput{ProjectID: "p1"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
}

func TestRegister_DuplicateNameErrors(t *testing.T) {
	t.Cleanup(Reset)
	if err := Register("dup", gcpcreds.CredInput{ProjectID: "p1"}); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := Register("dup", gcpcreds.CredInput{ProjectID: "p2"}); err == nil {
		t.Fatal("expected error on duplicate Register; got nil")
	}
}

func TestResolve_RoundTrip(t *testing.T) {
	t.Cleanup(Reset)
	want := gcpcreds.CredInput{
		ProjectID:          "rt-project",
		ServiceAccountJSON: []byte(`{"type":"service_account"}`),
	}
	if err := Register("rt", want); err != nil {
		t.Fatalf("Register: %v", err)
	}
	got, ok := Resolve("rt")
	if !ok {
		t.Fatal("Resolve(rt): not found")
	}
	if got.ProjectID != want.ProjectID || string(got.ServiceAccountJSON) != string(want.ServiceAccountJSON) {
		t.Errorf("round-trip = %+v, want %+v", got, want)
	}
}

func TestResolve_MissingReturnsZeroAndFalse(t *testing.T) {
	t.Cleanup(Reset)
	got, ok := Resolve("nope")
	if ok {
		t.Fatal("Resolve(nope): expected ok=false")
	}
	if got.ProjectID != "" || got.ServiceAccountJSON != nil {
		t.Errorf("missing entry should be zero-value, got %+v", got)
	}
}

func TestReset_ClearsRegistry(t *testing.T) {
	if err := Register("ephemeral", gcpcreds.CredInput{ProjectID: "x"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	Reset()
	if _, ok := Resolve("ephemeral"); ok {
		t.Error("Reset did not clear the entry")
	}
}

func TestConcurrentRegisterResolve_RaceClean(t *testing.T) {
	t.Cleanup(Reset)
	const N = 64
	var wg sync.WaitGroup
	wg.Add(N * 2)
	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			_ = Register(fmtName(i), gcpcreds.CredInput{ProjectID: fmtName(i)})
		}()
		go func() {
			defer wg.Done()
			_, _ = Resolve(fmtName(i))
		}()
	}
	wg.Wait()
}

func fmtName(i int) string {
	const hex = "0123456789abcdef"
	return "k-" + string([]byte{hex[(i>>4)&0xf], hex[i&0xf]})
}
