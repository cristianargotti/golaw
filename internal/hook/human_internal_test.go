package hook

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestHumanMessageNamesTheCauseOfACannotJudge holds the human channel to
// the zero-friction goal: a denial that names no check says which class of
// cause stopped the judgment, an input golaw could not read, one over the
// 8 MiB it reads or the internal deadline, instead of an unknown path.
func TestHumanMessageNamesTheCauseOfACannotJudge(t *testing.T) {
	t.Parallel()
	pre := model.Event{Shape: model.ShapeShell}
	for _, tc := range []struct {
		name, language, reason string
		cannot                 bool
		want                   string
	}{
		{"deadline", model.DefaultLanguage, "internal deadline passed: hook evaluation exceeded its budget", true, "dentro do prazo interno"},
		{"deadline-en", "en", "internal deadline passed: hook evaluation exceeded its budget", true, "within its internal deadline"},
		{"oversized", model.DefaultLanguage, "cannot judge: file larger than the read bound: input exceeds 8 MiB", true, "passa dos 8 MiB"},
		{"oversized-en", "en", "normalize hook input: cannot judge: the command is over the 8 MiB the hook reads: run it as smaller commands", true, "over the 8 MiB"},
		{"unreadable", model.DefaultLanguage, "cannot judge: read input: unexpected EOF", true, "não conseguiu ler o que precisa"},
		{"unreadable-en", "en", "normalize hook input: cannot judge: unsupported Codex tool Edit", true, "could not read what it needs"},
		{"a check", model.DefaultLanguage, "text.probe: cannot judge: the file is binary (a.go)", true, "pela verificação text.probe no caminho a.go"},
		{"a policy denial", model.DefaultLanguage, "load hook policy: enrollment mismatch", false, "pela verificação golaw no caminho não informado"},
	} {
		got := humanMessage(tc.language, pre, tc.reason, tc.cannot)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
		if tc.cannot && !strings.Contains(tc.reason, "text.probe") && strings.Contains(got, "não informado") {
			t.Errorf("%s names an unknown path: %q", tc.name, got)
		}
	}
	post := humanMessage(model.DefaultLanguage, model.Event{Shape: model.ShapePost}, "cannot judge: read input: unexpected EOF", true)
	if !strings.Contains(post, "não conseguiu ler o que precisa") {
		t.Fatalf("a post cannot-judge = %q", post)
	}
}
