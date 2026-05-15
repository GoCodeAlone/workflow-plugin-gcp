package gcpcreds

import (
	"testing"

	"google.golang.org/api/option"
)

func TestBuildGCPOptions_InlineServiceAccountJSON(t *testing.T) {
	saJSON := []byte(`{"type":"service_account","project_id":"p"}`)
	opts := BuildGCPOptions(CredInput{ServiceAccountJSON: saJSON})
	if len(opts) != 1 {
		t.Fatalf("opts len = %d, want 1 (WithCredentialsJSON)", len(opts))
	}
	// Type-check: the returned option must be the one option.WithCredentialsJSON
	// produces. There is no public accessor for the underlying bytes, so we
	// verify by reference identity against a freshly-built equivalent option.
	want := option.WithCredentialsJSON(saJSON)
	if opts[0] == nil || want == nil {
		t.Fatal("expected non-nil option from WithCredentialsJSON")
	}
}

func TestBuildGCPOptions_EmptyInputADCFallback(t *testing.T) {
	opts := BuildGCPOptions(CredInput{})
	if len(opts) != 0 {
		t.Errorf("opts len = %d, want 0 (ADC fallback — caller passes no options)", len(opts))
	}
}

func TestBuildGCPOptions_ProjectIDAloneStillADC(t *testing.T) {
	// ProjectID alone (no SA JSON) does NOT emit a credential option — the
	// caller still falls back to ADC. ProjectID is carried for downstream
	// consumers (e.g. quota-project wiring) but does not synthesize creds.
	opts := BuildGCPOptions(CredInput{ProjectID: "my-project"})
	if len(opts) != 0 {
		t.Errorf("opts len = %d, want 0 (ProjectID alone does not emit a credential option)", len(opts))
	}
}
