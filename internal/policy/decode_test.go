package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestDecode(t *testing.T) {
	t.Parallel()
	f := decode(t, fullPolicy)
	switch {
	case f.Schema != 1, f.Kit.Version != "0.1.0", f.Profile == nil || f.Profile.Name != "acme":
		t.Errorf("head = %+v", f)
	case f.Language == nil || *f.Language != "en-US", len(f.Capabilities) != 2:
		t.Errorf("language or capabilities = %+v", f)
	case f.Checks.Sizes == nil || *f.Checks.Sizes.FileLines != 250 || f.Checks.Sizes.FunctionLines == nil:
		t.Errorf("sizes = %+v", f.Checks.Sizes)
	case f.Checks.Coverage == nil || *f.Checks.Coverage.Total != 85 || len(f.Checks.Coverage.FileExemptions) != 1:
		t.Errorf("coverage = %+v", f.Checks.Coverage)
	case f.Checks.Terms == nil || len(f.Checks.Terms.Tools) != 2 || f.Checks.Lint == nil || f.Checks.Shell == nil:
		t.Errorf("terms, lint or shell = %+v", f.Checks)
	case f.Checks.Options["text.tool-terms"]["limit"] != 3:
		t.Errorf("options = %+v", f.Checks.Options)
	case f.Ladder == nil || !*f.Ladder.PreCommit || len(f.Waivers) != 1 || len(f.Exceptions) != 2 || len(f.Managed) != 1:
		t.Errorf("records = %+v", f)
	}
	minimal := decode(t, minimalPolicy)
	if minimal.Profile != nil || minimal.Language != nil || minimal.Checks.Sizes != nil || minimal.Checks.Options != nil {
		t.Errorf("absent keys must stay nil: %+v", minimal)
	}
}

func TestDecodeRefusals(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"unknown key":          minimalPolicy + "extra: 1\n",
		"unknown nested key":   minimalPolicy + "checks: {sizes: {file_lines: 1, lines: 2}}\n",
		"duplicate key":        minimalPolicy + "golaw: 1\n",
		"duplicate nested key": "golaw: 1\nkit: {version: 0.1.0, version: 0.2.0}\n",
		"duplicate check":      minimalPolicy + "checks: {text.x: {a: 1}, text.x: {a: 2}}\n",
		"wrong type":           minimalPolicy + "capabilities: go\n",
		"scalar option block":  minimalPolicy + "checks: {text.x: 5}\n",
		"empty":                "",
		"two documents":        minimalPolicy + "---\ngolaw: 1\n",
		"scalar document":      "hello\n",
		"not yaml":             "golaw: [\n",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := policy.Decode([]byte(text))
			if !errors.Is(err, model.ErrPolicy) || !strings.HasPrefix(err.Error(), "invalid policy: golaw.yml") {
				t.Errorf("Decode = %v, want ErrPolicy naming the file", err)
			}
		})
	}
}
