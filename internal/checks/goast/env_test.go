package goast_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
)

const envSource = "package a\n\nimport \"os\"\n\nfunc f() (string, bool) {\n\t_ = os.Environ()\n\treturn os.LookupEnv(\"HOME\")\n}\n"

func TestEnvRead(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.EnvRead(), map[string]string{
		"internal/app/a.go":      envSource,
		"internal/app/b.go":      "package app\n\nvar read = os.Getenv\n",
		"internal/app/a_test.go": envSource,
		"configs/a.go":           envSource,
		"cmd/app/a.go":           envSource,
	})
	expectCodes(t, res, "env-read", "env-read", "env-read")
	expectLines(t, res, 6, 7, 3)
	want := "os.Getenv reads the environment outside configs, cmd and a main package: add the knob to the configuration with a default"
	if res.Findings[2].Message != want {
		t.Errorf("message = %q", res.Findings[2].Message)
	}
}

func TestFindingsDenyWithPositions(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.EnvRead(), map[string]string{
		"internal/a.go": "package a\n\nimport \"os\"\n\nvar home = os.Getenv(\"HOME\")\n",
	})
	expectCodes(t, res, "env-read")
	f := res.Findings[0]
	if f.Severity != model.SeverityDeny || f.Path != "internal/a.go" || f.Line != 5 || f.Column != 12 {
		t.Errorf("finding = %+v", f)
	}
	if f.Check != "" || f.Rule != "" || f.Fingerprint != "" {
		t.Errorf("the engine fills check, rule and fingerprint, got %+v", f)
	}
}
