package lint_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/runner"
)

const shfmtMessage = "the file is not formatted as shfmt -i 2 -ci formats it"

func TestParseReadsTheInfrastructureFormats(t *testing.T) {
	t.Parallel()
	runParseCases(t, map[string]parseCase{
		"hadolint files": {lint.FormatHadolint, "hadolint", runner.Output{Stdout: recorded(t, lint.FormatHadolint, "findings.json"), Code: 1}, []lint.Issue{
			{Tool: "hadolint", Rule: "DL3018", Path: "deploy/valkey.Dockerfile", Line: 2, Column: 1, Message: "Pin versions in apk add. Instead of `apk add <package>` use `apk add <package>=<version>`", Severity: "warning"},
			{Tool: "hadolint", Rule: "DL3002", Path: "Dockerfile", Line: 3, Column: 1, Message: "Last USER should not be root", Severity: "warning"},
		}, nil},
		"hadolint stdin": {lint.FormatHadolint, "hadolint", runner.Output{Stdout: recorded(t, lint.FormatHadolint, "stdin.json"), Code: 1}, []lint.Issue{
			{Tool: "hadolint", Rule: "DL3002", Path: "", Line: 3, Column: 1, Message: "Last USER should not be root", Severity: "warning"},
		}, []string{"stdin"}},
		"hadolint clean": {lint.FormatHadolint, "hadolint", runner.Output{Stdout: []byte("[]\n")}, nil, nil},
		"shellcheck": {lint.FormatShellcheck, "shellcheck", runner.Output{Stdout: recorded(t, lint.FormatShellcheck, "findings.json"), Code: 1}, []lint.Issue{
			{Tool: "shellcheck", Rule: "SC2086", Path: "scripts/smoke.sh", Line: 2, Column: 6, Message: "Double quote to prevent globbing and word splitting.", Severity: "info"},
			{Tool: "shellcheck", Rule: "SC2164", Path: "scripts/smoke.sh", Line: 3, Column: 1, Message: "Use 'cd ... || exit' or 'cd ... || return' in case cd fails.", Severity: "warning"},
		}, nil},
		"shellcheck clean": {lint.FormatShellcheck, "shellcheck", runner.Output{Stdout: []byte(`{"comments":[]}`)}, nil, nil},
		"shfmt diff": {lint.FormatShfmt, "shfmt", runner.Output{Stdout: recorded(t, lint.FormatShfmt, "findings.diff"), Code: 1}, []lint.Issue{
			{Tool: "shfmt", Rule: "format", Path: "scripts/smoke.sh", Line: 3, Column: 1, Message: shfmtMessage, Severity: "error"},
		}, nil},
		"shfmt parse error": {lint.FormatShfmt, "shfmt", runner.Output{Stderr: []byte("scripts/broken.sh:4:1: reached EOF without closing quote '\"'\n"), Code: 1}, nil, []string{"scripts/broken.sh"}},
		"shfmt clean":       {lint.FormatShfmt, "shfmt", runner.Output{}, nil, nil},
		"cfn-lint": {lint.FormatCfnLint, "cfn-lint", runner.Output{Stdout: recorded(t, lint.FormatCfnLint, "findings.json"), Code: 2}, []lint.Issue{
			{Tool: "cfn-lint", Rule: "E3002", Path: "infra/10-base.yml", Line: 9, Column: 7, Message: "Additional properties are not allowed ('AccessControll' was unexpected)", Severity: "Error"},
		}, []string{"infra/20-app.yml"}},
		"cfn-lint warnings and notes": {lint.FormatCfnLint, "cfn-lint", runner.Output{Stdout: []byte("[]"), Code: 12}, nil, nil},
		"checkov one framework": {lint.FormatCheckov, "checkov", runner.Output{Stdout: recorded(t, lint.FormatCheckov, "cloudformation.json"), Code: 1}, []lint.Issue{
			{Tool: "checkov", Rule: "CKV_AWS_18", Path: "infra/10-base.yml", Line: 4, Message: "Ensure the S3 bucket has access logging enabled"},
		}, nil},
		"checkov two frameworks": {lint.FormatCheckov, "checkov", runner.Output{Stdout: recorded(t, lint.FormatCheckov, "frameworks.json"), Code: 1}, []lint.Issue{
			{Tool: "checkov", Rule: "CKV_AWS_21", Path: "infra/10-base.yml", Line: 4, Message: "Ensure the S3 bucket has versioning enabled", Severity: "LOW"},
			{Tool: "checkov", Rule: "CKV_DOCKER_2", Path: "Dockerfile", Line: 1, Message: "Ensure that HEALTHCHECK instructions have been added to container images"},
		}, []string{"infra/30-broken.yml"}},
		"checkov no file": {lint.FormatCheckov, "checkov", runner.Output{Stdout: recorded(t, lint.FormatCheckov, "empty.json")}, nil, nil},
		"yamllint": {lint.FormatYamllint, "yamllint", runner.Output{Stdout: recorded(t, lint.FormatYamllint, "findings.txt"), Code: 1}, []lint.Issue{
			{Tool: "yamllint", Rule: "document-start", Path: "deploy/values.yml", Line: 1, Column: 1, Message: "missing document start \"---\"", Severity: "warning"},
			{Tool: "yamllint", Rule: "line-length", Path: "api/openapi.yaml", Line: 12, Column: 81, Message: "line too long (95 > 80 characters)", Severity: "error"},
		}, []string{"deploy/broken.yml"}},
		"yamllint warnings only": {lint.FormatYamllint, "yamllint", runner.Output{Stdout: []byte("a.yml:1:1: [warning] truthy value should be one of [false, true] (truthy)\n"), Code: 2}, []lint.Issue{
			{Tool: "yamllint", Rule: "truthy", Path: "a.yml", Line: 1, Column: 1, Message: "truthy value should be one of [false, true]", Severity: "warning"},
		}, nil},
	})
}

func TestTheInfrastructureParsersRefuseWhatTheyCannotRead(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		format lint.Format
		out    runner.Output
		want   error
	}{
		"hadolint text output":        {lint.FormatHadolint, runner.Output{Stdout: []byte("Dockerfile:3 DL3002 warning: Last USER should not be root\n"), Code: 1}, lint.ErrFormat},
		"shellcheck json without key": {lint.FormatShellcheck, runner.Output{Stdout: []byte(`[]`), Code: 1}, lint.ErrFormat},
		"shellcheck bad option":       {lint.FormatShellcheck, runner.Output{Stderr: []byte("invalid option\n"), Code: 3}, lint.ErrToolFailed},
		"shfmt exit without output":   {lint.FormatShfmt, runner.Output{Code: 1}, lint.ErrFormat},
		"shfmt text that is no diff":  {lint.FormatShfmt, runner.Output{Stdout: []byte("formatted\n"), Code: 1}, lint.ErrFormat},
		"cfn-lint crashed":            {lint.FormatCfnLint, runner.Output{Stderr: []byte("Traceback (most recent call last):\n"), Code: 1}, lint.ErrToolFailed},
		"cfn-lint odd status":         {lint.FormatCfnLint, runner.Output{Stdout: []byte("[]"), Code: 3}, lint.ErrToolFailed},
		"checkov text output":         {lint.FormatCheckov, runner.Output{Stdout: []byte("Passed checks: 3, Failed checks: 1\n"), Code: 1}, lint.ErrFormat},
		"yamllint unknown line":       {lint.FormatYamllint, runner.Output{Stdout: []byte("everything is fine\n"), Code: 1}, lint.ErrFormat},
		"yamllint usage error":        {lint.FormatYamllint, runner.Output{Stderr: []byte("usage: yamllint\n"), Code: 4}, lint.ErrToolFailed},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := lint.Parse(tc.format, "tool", tc.out, root)
			if !errors.Is(err, tc.want) || got != nil {
				t.Errorf("Parse = %+v, %v, want %v and no issue", got, err, tc.want)
			}
		})
	}
}
