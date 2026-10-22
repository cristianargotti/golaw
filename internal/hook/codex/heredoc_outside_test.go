package codex_test

import (
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/codex"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
)

// A here-document written outside the repository, such as the scratch
// script an agent writes before it runs it, is no write of this
// repository, and an operator the recognizer does not read stays the
// shell family's: neither is refused as a write the hook cannot judge.
func TestHeredocOutsideTheRecognizerKeepsTheShellRoute(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, command string
	}{
		{"cat outside", "cat > ../scratch.sh <<'EOF'\nprintf hello\nEOF"},
		{"cat outside last", "cat <<'EOF' > ../scratch.sh\nprintf hello\nEOF"},
		{"append outside", "cat >> ../notes.md <<'EOF'\nhello\nEOF"},
		{"home target", "cat > ~/notes.md <<'EOF'\nhello\nEOF"},
		{"tab stripping", "cat > notes.md <<-EOF\n\thello\n\tEOF"},
		{"here string", "cat > notes.md <<< hello"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := commandPayload(t, "pre-bash-plain.json", tt.command)
			request := decodeRequest(t, data, nil)
			if request.Writes != nil || request.Event.Shape != model.ShapeShell {
				t.Fatalf("became a write: %+v", request.Writes)
			}
			if _, result := hooktest.Evaluate(t, "codex", adapter(), data, nil); result.Outcome != hook.Allow {
				t.Fatalf("refused: %+v", result)
			}
		})
	}
}

// A patch Codex applies itself is judged where Codex writes it: its target
// is the first path it writes, joined to the working directory and the cd
// word literally, as Codex joins them, so enrollment answers for that
// location. A location no repository enrolls keeps the valid allow, and a
// cd word that starts with ~ names a directory under the working
// directory, never the home.
func TestHeredocPatchIsJudgedWhereCodexWritesIt(t *testing.T) {
	t.Parallel()
	dash := string(rune(0x2014))
	root, other := hooktest.Repository(t), hooktest.Repository(t)
	body := patch("*** Add File: notes.md\n+hello " + dash + " world\n")
	outside := patch("*** Add File: ../outside.md\n+hello " + dash + " world\n")
	for _, tt := range []struct {
		name, command, target string
		want                  hook.Outcome
	}{
		{"cd outside", "cd .. && apply_patch <<'EOF'\n" + body + "EOF", filepath.Join(filepath.Dir(root), "notes.md"), hook.Allow},
		{"cd another directory", "cd '" + filepath.ToSlash(other) + "' && apply_patch <<'EOF'\n" + body + "EOF", filepath.Join(other, "notes.md"), hook.Allow},
		{"patch outside", "apply_patch <<'EOF'\n" + outside + "EOF", filepath.Join(filepath.Dir(root), "outside.md"), hook.Allow},
		{"cd tilde", "cd ~/sub && apply_patch <<'EOF'\n" + body + "EOF", filepath.Join(root, "~", "sub", "notes.md"), hook.Deny},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := hooktest.Payload(commandPayload(t, "pre-bash-plain.json", tt.command), root)
			parsed, err := codex.Parse(data)
			if err != nil || parsed.Event.Target != tt.target {
				t.Fatalf("target = %q, %v; want %q", parsed.Event.Target, err, tt.target)
			}
			_, result := hook.Evaluate(t.Context(), "codex", data, adapter(), enrolledUnder(t, root, nil))
			if result.Outcome != tt.want {
				t.Fatalf("want %s, got %+v", tt.want, result)
			}
		})
	}
}
