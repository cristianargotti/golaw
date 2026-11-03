package rulesdoc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rulesdoc"
)

const page = "# Rules\r\n\r\n| Inventory row | Delivered location |\n| --- | --- |\n| 4.1: one use case | agents/SKILL.md |\n" +
	"| Check | Rule | Needs | Why and boundary |\n| --- | --- | --- | --- |\n" +
	"| commit.shape | rules/4.12@1 | none | A commit too large, because one theme. |\r\n" +
	"| shell.shape | rules/4.12@1 | none | A staged | piped shape, because one theme. |\n" +
	"| goast.env-read | rules/4.3@1 | go | " + "An environment read " + "outside configs and a composition root and every other place that is not the edge of the process, because configuration enters once. |\n" +
	"| Reader | File | Declared subset | Stated limit | How to resolve |\n| readMakefile | build_makefile.go | x | y | z |\n" +
	"| broken | rules/4.1@1 | none |\n"

func TestParseRulesKeepsOnlyRuleRows(t *testing.T) {
	t.Parallel()
	rules := rulesdoc.ParseRules([]byte(page))
	ids := make([]string, 0, len(rules))
	for _, rule := range rules {
		ids = append(ids, rule.ID)
	}
	if got := strings.Join(ids, ","); got != "commit.shape,shell.shape,goast.env-read" {
		t.Fatalf("ids = %s", got)
	}
	if rules[1].Why != "A staged | piped shape, because one theme." {
		t.Fatalf("a pipe inside the why cell splits it: %q", rules[1].Why)
	}
}

func TestSelectByIdFamilyAndReference(t *testing.T) {
	t.Parallel()
	rules := rulesdoc.ParseRules([]byte(page))
	for _, row := range []struct {
		query, want string
	}{
		{"commit.shape", "commit.shape"},
		{"goast", "goast.env-read"},
		{"rules/4.12", "commit.shape,shell.shape"},
		{"shell.sh", "shell.shape"},
	} {
		kept, err := rulesdoc.Select(rules, row.query)
		if err != nil {
			t.Fatalf("%s: %v", row.query, err)
		}
		ids := make([]string, 0, len(kept))
		for _, rule := range kept {
			ids = append(ids, rule.ID)
		}
		if got := strings.Join(ids, ","); got != row.want {
			t.Errorf("%s = %s, want %s", row.query, got, row.want)
		}
	}
}

func TestSelectRefusesAQueryNoRuleStarts(t *testing.T) {
	t.Parallel()
	rules := rulesdoc.ParseRules([]byte(page))
	// The near miss is a suffix of an id, which names no family.
	for _, query := range []string{"shape", "readMakefile", "4.12"} {
		if _, err := rulesdoc.Select(rules, query); !errors.Is(err, rulesdoc.ErrNoRule) {
			t.Errorf("%s: err = %v", query, err)
		}
	}
}

func TestRuleLines(t *testing.T) {
	t.Parallel()
	rules := rulesdoc.ParseRules([]byte(page))
	if got := rules[0].Line(); got != "commit.shape rules/4.12@1 needs none: A commit too large, because one theme." {
		t.Fatalf("line = %q", got)
	}
	if got := rules[0].IndexLine(); got != "commit.shape: A commit too large" {
		t.Fatalf("index = %q", got)
	}
	long := rules[2].IndexLine()
	if !strings.HasSuffix(long, "...") || len([]rune(long)) > len("goast.env-read: ")+93 {
		t.Fatalf("long index line is not bounded: %q", long)
	}
}
