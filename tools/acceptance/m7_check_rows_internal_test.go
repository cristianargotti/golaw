package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
)

// m7RunCheck runs one check row by name over replies put ahead of a
// pipeline that agrees with the fresh repository.
func m7RunCheck(t *testing.T, rendered map[string][]byte, ids []string, name string, first ...func(*m7Suite) m5Reply) error {
	t.Helper()
	s := m7FreshSuite(t)
	replies := make([]m5Reply, 0, len(first))
	for _, reply := range first {
		replies = append(replies, reply(s))
	}
	s.execute = m5Script(t, s.suite, append(replies, m7PipelineReplies(t, s, rendered, ids)...)...).Execute
	for _, c := range m7CheckCases() {
		if c.name == name {
			return c.run(s.m7Scope(m7CheckCategory, c.name), t.Context())
		}
	}
	t.Fatalf("no check row %s", name)
	return nil
}

// m7Checked answers golaw gen --check with got, once when once is set.
func m7Checked(got result, once bool) func(*m7Suite) m5Reply {
	return func(*m7Suite) m5Reply {
		return m5Reply{prefix: []string{"gen", "--check"}, once: once, answer: m5Answer(got)}
	}
}

func TestM7CheckRowsPassWhenTheCheckNamesEachDriftAndItsRepair(t *testing.T) {
	t.Parallel()
	rendered, ids := m7Rendered(t), m7FillIDsOf(t)
	clean := m7Checked(result{stdout: m7CheckClean + "\n"}, false)
	for _, tc := range []struct {
		row, path, cause string
	}{
		{"drift-edited", m7FakePath, gen.DriftDiffers},
		{"drift-missing", m7PlansPath, gen.DriftMissing},
		{"drift-third-party", m7QueriesPath, gen.DriftThirdParty},
	} {
		drift := m7Checked(result{code: 1, stdout: m7DriftLine(tc.path, tc.cause) + "\n"}, true)
		if err := m7RunCheck(t, rendered, ids, tc.row, drift, clean); err != nil {
			t.Fatalf("%s: %v", tc.row, err)
		}
	}
	if err := m7RunCheck(t, rendered, ids, "clean-after-fill", clean); err != nil {
		t.Fatalf("clean-after-fill: %v", err)
	}
	if err := m7RunCheck(t, rendered, ids, "open-hole"); err != nil {
		t.Fatalf("open-hole: %v", err)
	}
}

// TestM7CheckRowsFailOnACheckThatMissesWhatItMustName: an unnamed drift,
// a repair golaw gen refuses, a repair that leaves drift, a hole the check
// leaves out and a repository that could not be built each fail the row.
func TestM7CheckRowsFailOnACheckThatMissesWhatItMustName(t *testing.T) {
	t.Parallel()
	rendered, ids := m7Rendered(t), m7FillIDsOf(t)
	drift := m7Checked(result{code: 1, stdout: m7DriftLine(m7FakePath, gen.DriftDiffers) + "\n"}, true)
	gens := 0
	refuseRepair := func(*m7Suite) m5Reply {
		return m5Reply{prefix: []string{"gen"}, answer: func(t *testing.T, c command) result {
			t.Helper()
			gens++
			if gens > 1 {
				return result{code: 2, stderr: "golaw: refused\n"}
			}
			m2Write(t, filepath.Join(c.dir, filepath.FromSlash(m7QueriesPath)), "package sqlcgen\n")
			return result{}
		}}
	}
	for _, tc := range []struct {
		name, row, want string
		first           []func(*m7Suite) m5Reply
	}{
		{"red the drift unnamed", "drift-edited", "lacks", []func(*m7Suite) m5Reply{m7Checked(result{code: 1, stdout: "golaw gen --check: 1 drift, 0 open holes\n"}, false)}},
		{"red the repair refused", "drift-edited", "repair the drift", []func(*m7Suite) m5Reply{drift, refuseRepair}},
		{"red the repair left drift", "drift-edited", "exit 1, want 0", []func(*m7Suite) m5Reply{drift, m7Checked(result{code: 1}, false)}},
		{"red a hole left out", "open-hole", "lacks", []func(*m7Suite) m5Reply{m7Checked(result{code: 1, stdout: m7HoleLine(ids[0]) + "\n"}, false)}},
		{"red the repository unbuilt", "clean-after-fill", "tidy the module", []func(*m7Suite) m5Reply{func(s *m7Suite) m5Reply {
			return m5Reply{program: s.goTool, prefix: []string{"mod", "tidy"}, answer: m5Answer(result{code: 1})}
		}}},
	} {
		gens = 0
		err := m7RunCheck(t, rendered, ids, tc.row, tc.first...)
		if !errors.Is(err, ErrAcceptance) || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: %v, want an error naming %q", tc.name, err, tc.want)
		}
	}
}

func TestM7DriftEditsNameAFileThatIsNotThere(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "absent.go")
	if err := m7AppendEdit(missing); err == nil {
		t.Fatal("a hand edit of a missing file passed")
	}
	if err := m7Remove(missing); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), "absent.go") {
		t.Fatalf("removing a missing file = %v", err)
	}
}
