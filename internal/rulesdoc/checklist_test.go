package rulesdoc_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/rulesdoc"
)

const checklists = "# Engineering checklists\n\nIntro.\n\n## Builder\n\n### Before changing code\n\n- read\n\n" +
	"### While implementing\n\n- test first\n\n## Reviewer\n\n- judge\n\n## Conditional checks\n\n" +
	"### APIs and authentication\r\n\r\n- tokens\r\n\r\n#### Deeper\n\n- stays inside\n"

func TestParseSectionsNamesAndBounds(t *testing.T) {
	t.Parallel()
	sections := rulesdoc.ParseSections([]byte(checklists))
	names := make([]string, 0, len(sections))
	for _, section := range sections {
		names = append(names, section.Name)
	}
	want := "builder,before-changing-code,while-implementing,reviewer,conditional-checks,apis-and-authentication"
	if got := strings.Join(names, ","); got != want {
		t.Fatalf("names = %s", got)
	}
	builder := strings.Join(sections[0].Lines, "\n")
	if !strings.Contains(builder, "- test first") || strings.Contains(builder, "Reviewer") {
		t.Fatalf("builder holds its subsections and stops at the next section: %q", builder)
	}
	if got := strings.Join(sections[1].Lines, "\n"); got != "### Before changing code\n\n- read" {
		t.Fatalf("subsection = %q", got)
	}
	if got := strings.Join(sections[5].Lines, "\n"); !strings.Contains(got, "- stays inside") {
		t.Fatalf("a deeper heading ends the section: %q", got)
	}
}

func TestFindExactAndUniquePrefix(t *testing.T) {
	t.Parallel()
	sections := rulesdoc.ParseSections([]byte(checklists))
	for _, row := range []struct{ name, want string }{
		{"reviewer", "reviewer"},
		{"apis", "apis-and-authentication"},
		{"builder", "builder"},
	} {
		section, err := rulesdoc.Find(sections, row.name)
		if err != nil || section.Name != row.want {
			t.Errorf("%s = %q, %v", row.name, section.Name, err)
		}
	}
}

func TestFindListsTheNamesWhenUnknownOrAmbiguous(t *testing.T) {
	t.Parallel()
	sections := rulesdoc.ParseSections([]byte(checklists))
	for _, name := range []string{"nope", "b", ""} {
		_, err := rulesdoc.Find(sections, name)
		if !errors.Is(err, rulesdoc.ErrNoChecklist) || !strings.Contains(err.Error(), "builder, before-changing-code") {
			t.Errorf("%q: err = %v", name, err)
		}
	}
}

func TestSlugDropsPunctuation(t *testing.T) {
	t.Parallel()
	if got := rulesdoc.Slug("  Service configuration, and Operations (v2)"); got != "service-configuration-and-operations-v2" {
		t.Fatalf("slug = %q", got)
	}
}
