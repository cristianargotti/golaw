package model_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestTheSonarKeyStaysOutOfTheFingerprint(t *testing.T) {
	t.Parallel()
	plain := model.Finding{Check: "lint.eslint", Rule: "eslint/sonarjs/no-unenclosed-multiline-block", Path: "views/panel.html", Code: "sonarjs-no-unenclosed-multiline-block", Message: "This line will not be executed conditionally."}
	mapped := plain
	mapped.Sonar = "javascript:S2681"
	if model.Fingerprint(plain) != model.Fingerprint(mapped) {
		t.Error("a Sonar key changed the fingerprint: the staged multiset would see a new finding")
	}
}

func TestTheSonarKeyIsWrittenOnlyWhenMapped(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		sonar string
		want  bool
	}{
		"a mapped finding":                {sonar: "python:S2068", want: true},
		"an unmapped finding (near miss)": {sonar: "", want: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(model.Finding{Check: "lint.ruff", Code: "s105", Severity: model.SeverityFail, Sonar: tc.sonar})
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(data), `"sonar":`); got != tc.want {
				t.Errorf("json %s carries the sonar key = %v, want %v", data, got, tc.want)
			}
		})
	}
}
