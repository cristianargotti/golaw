package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestM2AdoptionRejectsFalseCommitEvidence(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"no-commit", "finding", "invalid-pin", "old-pin", "missing-head"} {
		s := m2TestSuite(t)
		script := m2LifecycleScript(t, s, "dispatch-unarmed", "answer")
		s.execute = m2AdoptionFault(s, script, mode)
		if err := s.lifecycleM2(t.Context(), "dispatch-unarmed"); err == nil {
			t.Fatalf("%s: invalid commit evidence passed", mode)
		}
	}
}

func m2CorruptCommittedEvidence(c command, got result, mode string) result {
	if slices.Contains(c.args, "HEAD:"+policyFile) {
		switch mode {
		case "invalid-pin":
			got.stdout = "["
		case "old-pin":
			got.stdout = strings.Replace(got.stdout, m2CandidateVersion, m2OlderVersion, 1)
		}
	}
	if mode == "missing-head" && slices.Contains(c.args, "--verify") {
		return result{err: ErrAcceptance}
	}
	return got
}

func TestM2UpdateRejectsPreviewAndApplyMutation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"preview", "edit", "enrollment", "read-edit"} {
		s := m2TestSuite(t)
		script := m2LifecycleScript(t, s, "update-preserves-edit", "answer")
		s.execute = func(ctx context.Context, c command) result {
			got := script.Execute(ctx, c)
			if m2Kind(s, c) != "update" {
				return got
			}
			apply := slices.Contains(c.args, flagApply)
			switch {
			case mode == "preview" && !apply:
				m2Write(t, filepath.Join(c.dir, pathNote), "preview wrote a file\n")
			case mode == "edit" && apply:
				m2Write(t, filepath.Join(c.dir, "golaw.mk"), "replaced edit\n")
			case mode == "enrollment" && apply:
				m2Pin(t, c.dir, m2OlderVersion)
			case mode == "read-edit" && apply:
				got = result{err: ErrAcceptance}
			}
			return got
		}
		if err := s.lifecycleM2(t.Context(), "update-preserves-edit"); err == nil {
			t.Fatalf("%s: mutation passed", mode)
		}
	}
}

func TestM2AdoptionPropagatesCommandFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		row, kind, word string
		at              int
	}{
		{"dispatch-unarmed", "probe", "", 1},
		{"dispatch-unarmed", "git", gitAdd, 1},
		{"dispatch-unarmed", "git", "--git-path", 1},
		{"dispatch-unarmed", "init", "", 1},
		{"dispatch-unarmed", "git", "--verify", 1},
		{"dispatch-unarmed", "git", gitAdd, 2},
		{"dispatch-unarmed", "git", "HEAD:" + policyFile, 1},
		{"upgrade-commit", "setup", "", 1},
		{"upgrade-commit", "git", "write-tree", 1},
		{"upgrade-commit", "update", "", 1},
		{"update-preserves-edit", "git", "write-tree", 1},
		{"update-preserves-edit", "update", "", 1},
		{"update-preserves-edit", "update", "", 2},
	} {
		s := m2TestSuite(t)
		script := m2LifecycleScript(t, s, tc.row, "answer")
		seen := 0
		s.execute = func(ctx context.Context, c command) result {
			if m2Kind(s, c) == tc.kind && (tc.word == "" || slices.Contains(c.args, tc.word)) {
				seen++
				if seen == tc.at {
					return result{err: ErrAcceptance}
				}
			}
			return script.Execute(ctx, c)
		}
		if err := s.lifecycleM2(t.Context(), tc.row); err == nil || seen != tc.at {
			t.Fatalf("%+v: failure=%v reached=%d", tc, err, seen)
		}
	}
}

func TestM2AdoptionRequiresPreparationAndReadableFiles(t *testing.T) {
	t.Parallel()
	s, repo := m2LifecycleFixture(t, "dispatch-unarmed", false)
	s.m2.Ready = false
	if err := s.m2DispatchUnarmed(t.Context(), repo); err == nil {
		t.Fatal("unverified tools passed")
	}
	s.m2.Ready = true
	s.lookup = func(string) (string, error) { return "", ErrAcceptance }
	if err := s.m2DispatchUnarmed(t.Context(), repo); err == nil {
		t.Fatal("unavailable prerequisites passed")
	}
	if err := s.m2UpdatePreview(t.Context(), filepath.Join(repo, "missing")); err == nil {
		t.Fatal("missing preview tree passed")
	}
	if err := s.m2CommittedPin(t.Context(), repo); err == nil {
		t.Fatal("unborn committed policy passed")
	}
}

func TestM2UpdateRequiresTheNamedLocalEdit(t *testing.T) {
	t.Parallel()
	for _, apply := range []bool{false, true} {
		s := m2TestSuite(t)
		script := m2LifecycleScript(t, s, "update-preserves-edit", "answer")
		s.execute = func(ctx context.Context, c command) result {
			got := script.Execute(ctx, c)
			if m2Kind(s, c) == "update" && slices.Contains(c.args, flagApply) == apply {
				got.stdout = "update completed\n"
			}
			return got
		}
		if err := s.lifecycleM2(t.Context(), "update-preserves-edit"); err == nil {
			t.Fatalf("apply=%v: unnamed local edit passed", apply)
		}
	}
}

func m2AdoptionFault(s *suite, script *m2Script, mode string) executor {
	committed := false
	return func(ctx context.Context, c command) result {
		through := m2Kind(s, c) == "commit" && slices.Contains(c.args, "feat: adopt the golaw policy")
		if through && mode == "no-commit" {
			return result{}
		}
		got := script.Execute(ctx, c)
		if through {
			committed = true
			if mode == "finding" {
				got.stdout = "text.dashes"
			}
		}
		if committed && c.program == s.git {
			got = m2CorruptCommittedEvidence(c, got, mode)
		}
		return got
	}
}

func TestM2UpdatePreservesManagedMakefileEdit(t *testing.T) {
	t.Parallel()
	s := m2TestSuite(t)
	script := m2LifecycleScript(t, s, "update-preserves-edit", "answer")
	var repo string
	var edited []byte
	updates := 0
	s.execute = func(ctx context.Context, c command) result {
		if m2Kind(s, c) != "update" {
			return script.Execute(ctx, c)
		}
		repo = c.dir
		body, err := os.ReadFile(filepath.Join(repo, "golaw.mk"))
		if err != nil || !bytes.HasSuffix(body, []byte(m2LocalEdit)) {
			t.Fatalf("managed golaw.mk lacks the local edit: %v", err)
		}
		if edited == nil {
			edited = bytes.Clone(body)
		}
		updates++
		got := script.Execute(ctx, c)
		if !strings.Contains(got.stdout+got.stderr, "golaw.mk") {
			t.Fatal("update did not name the edited managed asset")
		}
		return got
	}
	if err := s.lifecycleM2(t.Context(), "update-preserves-edit"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(repo, "golaw.mk"))
	if err != nil || !bytes.Equal(edited, after) || updates != 2 {
		t.Fatalf("managed edit changed or preview/apply missing: updates=%d error=%v", updates, err)
	}
	body, err := os.ReadFile(filepath.Join(repo, policyFile))
	if err != nil {
		t.Fatal(err)
	}
	m2RequirePinRecord(t, s, repo, m2CandidateVersion, string(body))
}
