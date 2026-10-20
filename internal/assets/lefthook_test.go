package assets_test

import (
	"testing"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
)

func TestLefthookAssetHasExactlyTheThreeBinaryCalls(t *testing.T) {
	t.Parallel()
	const want = `# golaw rungs, opt-in: nothing here runs until a person runs golaw setup,
# each rung switches off in the ladder of golaw.yml without a finding, and
# remote CI or the platform pipeline is the only gate.
min_version: 2.1.0
pre-commit:
  commands:
    golaw:
      run: golaw hook pre-commit
commit-msg:
  commands:
    golaw:
      run: golaw hook commit-msg {1}
pre-push:
  commands:
    golaw:
      run: golaw hook pre-push {1} {2}
      use_stdin: true
`
	entries, err := assets.Templates(golaw.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Destination != "lefthook.yml" {
			continue
		}
		got, renderErr := assets.Render(golaw.FS, entry, templateValues())
		if renderErr != nil {
			t.Fatal(renderErr)
		}
		if string(got.Data) != want || entry.Ownership != assets.OwnedByKit || len(entry.When) != 0 {
			t.Fatalf("unexpected ladder asset:\n%s", got.Data)
		}
		return
	}
	t.Fatal("lefthook template missing")
}
