package policyfiles_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

type openapiSnapshot struct {
	*proof.Tree

	failures map[string]error
	reads    map[string]int
}

func (s *openapiSnapshot) Read(file string) ([]byte, error) {
	s.reads[file]++
	if err := s.failures[file]; err != nil {
		return nil, err
	}
	data, err := s.Tree.Read(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	return data, nil
}

func openapiSnapshotFixture(t *testing.T) *openapiSnapshot {
	t.Helper()
	base := depsProof(t, "policy.openapi-assertions", "near", "api")
	files := make(map[string][]byte, len(base.files))
	for file, data := range base.files {
		files[file] = []byte(data)
	}
	files["api/openapi.yaml"] = []byte("openapi: 3.0.3\ncomponents: {schemas: {" +
		"Amount: {$ref: './schemas.yaml#/Amount'}, Again: {$ref: './schemas.yaml#/Amount'}}}\n")
	files["api/schemas.yaml"] = []byte("Amount: {type: number, format: float}\n")
	return &openapiSnapshot{Tree: proof.NewTree(files), reads: make(map[string]int)}
}

func TestOpenAPISnapshotReadFailures(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"redocly.yaml", "api/openapi.yaml", "api/schemas.yaml", ".redocly.lint-ignore.yaml"} {
		for _, failure := range []error{model.ErrUnreadable, model.ErrTooLarge} {
			t.Run(file+failure.Error(), func(t *testing.T) {
				t.Parallel()
				snapshot := openapiSnapshotFixture(t)
				snapshot.failures = map[string]error{file: failure}
				got, err := policyfiles.OpenAPIAssertions().Run(t.Context(), model.Input{Snapshot: snapshot})
				wantStatus := model.StatusUnmeasured
				var unread []string
				if errors.Is(failure, model.ErrTooLarge) {
					wantStatus, unread = model.StatusMeasured, []string{file}
				}
				if err != nil || got.Status.Kind != wantStatus || len(got.Findings) != 0 || !slices.Equal(got.Unread, unread) {
					t.Fatalf("result = %+v, error = %v, want status %s unread %v", got, err, wantStatus, unread)
				}
			})
		}
	}
}

func TestOpenAPIReferenceEvidenceIsMemoized(t *testing.T) {
	t.Parallel()
	snapshot := openapiSnapshotFixture(t)
	got, err := policyfiles.OpenAPIAssertions().Run(t.Context(), model.Input{Snapshot: snapshot})
	if err != nil || got.Status.Kind != model.StatusMeasured || len(got.Findings) != 1 {
		t.Fatalf("result = %+v, error = %v", got, err)
	}
	if snapshot.reads["api/schemas.yaml"] != 1 || snapshot.reads["api/openapi.yaml"] != 1 {
		t.Fatalf("reference reads = %+v", snapshot.reads)
	}
	item := got.Findings[0]
	if item.Path != "api/schemas.yaml" || item.Line != 1 {
		t.Fatalf("reference location = %+v", item)
	}
}
