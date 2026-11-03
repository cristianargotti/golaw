package support_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gentest"
)

// supportParts answers the parts a render holds: a subpackage, or root.
func supportParts(out genkind.Output) map[string]bool {
	parts := map[string]bool{}
	for _, f := range out.Regenerable {
		rest := strings.TrimPrefix(f.Path, "internal/gensupport/")
		if dir, _, ok := strings.Cut(rest, "/"); ok {
			parts[dir] = true
			continue
		}
		parts[rest] = true
	}
	return parts
}

func renderWith(t *testing.T, files map[string][]byte) map[string]bool {
	t.Helper()
	in := input()
	in.Snapshot = gentest.Snapshot(t, files)
	out, err := support.Kind().New(t.Context(), in, genkind.Request{})
	if err != nil {
		t.Fatal(err)
	}
	return supportParts(out)
}

const portTest = `package growth_test

import (
	"example.com/slice/internal/gensupport"
	"example.com/slice/internal/gensupport/porttest"
)

var _ = porttest.Run
var _ = gensupport.NewRecorder
`

// A repository that uses only the port suite gets that part and the
// recorder, never the HTTP, TLS or SQL parts it does not call.
func TestSupportWritesOnlyThePartsTheRepositoryUses(t *testing.T) {
	t.Parallel()
	parts := renderWith(t, map[string][]byte{"go.mod": []byte("module example.com/slice\n"), "internal/application/growth/cases_test.go": []byte(portTest)})
	for _, want := range []string{"porttest", "zz_generated_recorder.go", "zz_generated_recorder_test.go"} {
		if !parts[want] {
			t.Errorf("parts = %v, want %s", parts, want)
		}
	}
	for _, unused := range []string{"sqltest", "tlstest", "zz_generated_httpx.go", "zz_generated_errors_httpx.go"} {
		if parts[unused] {
			t.Errorf("parts = %v, %s is not used", parts, unused)
		}
	}
}

// Near rows: a part already in the repository is kept, so a run never
// shrinks the package, and a repository that uses nothing yet, as on golaw
// new support, gets the whole package.
func TestSupportKeepsWhatIsThereAndWritesAllWhenNothingIsUsed(t *testing.T) {
	t.Parallel()
	kept := renderWith(t, map[string][]byte{
		"go.mod": []byte("module example.com/slice\n"),
		"internal/application/growth/cases_test.go":          []byte(portTest),
		"internal/gensupport/sqltest/zz_generated_driver.go": []byte("package sqltest\n"),
	})
	if !kept["sqltest"] || kept["tlstest"] {
		t.Errorf("parts = %v, want sqltest kept and tlstest absent", kept)
	}
	all := renderWith(t, map[string][]byte{"go.mod": []byte("module example.com/slice\n")})
	for _, want := range []string{"porttest", "sqltest", "tlstest", "zz_generated_httpx.go"} {
		if !all[want] {
			t.Errorf("parts = %v, want %s in the whole package", all, want)
		}
	}
}
