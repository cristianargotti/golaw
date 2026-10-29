package bootstrap_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func freshPolicy(t *testing.T) []byte {
	t.Helper()
	in, err := bootstrap.Inputs{
		KitVersion: "9.9.9", Language: model.DefaultLanguage,
		Layout: bootstrap.Layout{Main: bootstrap.MainRoot, Decisions: "docs/adr", Go: true},
	}.Normalize()
	if err != nil {
		t.Fatal(err)
	}
	data, err := bootstrap.Render(in)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func rulesOf(t *testing.T, data []byte) string {
	t.Helper()
	file, err := policy.Decode(data)
	if err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
	if file.Surface == nil || file.Surface.Rules == nil {
		return ""
	}
	return *file.Surface.Rules
}

func TestSetSurfaceRulesWritesTheOneFlowLine(t *testing.T) {
	t.Parallel()
	fresh := freshPolicy(t)
	strict, err := bootstrap.SetSurfaceRules(fresh, model.SurfaceStrict)
	if err != nil || !bytes.HasSuffix(strict, []byte("\nsurface: { rules: strict }\n")) || rulesOf(t, strict) != "strict" {
		t.Fatalf("SetSurfaceRules = %q, %v", strict, err)
	}
	parity, err := bootstrap.SetSurfaceRules(strict, model.SurfaceParity)
	if err != nil || rulesOf(t, parity) != "parity" || strings.Count(string(parity), "surface:") != 1 {
		t.Fatalf("replace = %q, %v", parity, err)
	}
	same, err := bootstrap.SetSurfaceRules(parity, model.SurfaceParity)
	if err != nil || !bytes.Equal(same, parity) {
		t.Errorf("an unchanged floor rewrote the file: %v", err)
	}
	trimmed := bytes.TrimSuffix(fresh, []byte("\n"))
	appended, err := bootstrap.SetSurfaceRules(trimmed, model.SurfaceStrict)
	if err != nil || rulesOf(t, appended) != "strict" {
		t.Errorf("a file without a final newline = %q, %v", appended, err)
	}
}

func TestSetSurfaceRulesLeavesOtherFormsToThePerson(t *testing.T) {
	t.Parallel()
	fresh := freshPolicy(t)
	if _, err := bootstrap.SetSurfaceRules(fresh, "lenient"); !errors.Is(err, model.ErrUsage) {
		t.Errorf("an unknown set = %v, want ErrUsage", err)
	}
	block := append(bytes.Clone(fresh), []byte("surface:\n  rules: parity\n")...)
	_, err := bootstrap.SetSurfaceRules(block, model.SurfaceStrict)
	if !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), "surface: { rules: strict }") {
		t.Errorf("a block form = %v, want the line to write by hand", err)
	}
	if _, err := bootstrap.SetSurfaceRules([]byte("kit: [\n"), model.SurfaceStrict); err == nil {
		t.Error("a policy that does not decode was edited")
	}
}

func TestSetSurfaceRulesKeepsTheKeysAFlowLineCarries(t *testing.T) {
	t.Parallel()
	richer := append(freshPolicy(t), []byte("surface: { rules: parity, scripts: { globals: [jQuery] } }\n")...)
	if _, err := bootstrap.SetSurfaceRules(richer, model.SurfaceStrict); !errors.Is(err, model.ErrUsage) {
		t.Errorf("a flow line with more than the floor = %v, want the line to write by hand", err)
	}
}
