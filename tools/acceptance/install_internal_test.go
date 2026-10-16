package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstalledInvocation(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	if !strings.Contains(s.binary, " ") || !strings.Contains(s.binary, "área") ||
		s.digest == "" || s.hookFile == "" {
		t.Fatalf("%+v", s)
	}
	repo, err := s.repository(t.Context(), "control", true)
	if err != nil {
		t.Fatal(err)
	}
	input := []byte(`{"tool_input":{"content":"safe"}}`)
	called := false
	s.execute = func(_ context.Context, c command) result {
		called = true
		if c.program != s.binary || c.argv0 != binaryName ||
			strings.Join(c.args, " ") != "hook claude" ||
			c.dir != repo || c.input != string(input) {
			t.Fatalf("%+v", c)
		}
		return result{}
	}
	for _, event := range []string{eventPre, eventPost} {
		for _, tool := range []string{toolWrite, toolEdit} {
			err := expect(s.invoke(t.Context(), repo, event, tool, input), wantAllow)
			assertPass(t, measured("invocation", event+"/"+tool, err))
		}
	}
	if !called {
		t.Fatal("installed invocation was not executed")
	}
	if _, err := s.repository(t.Context(), "unenrolled", false); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationFailures(t *testing.T) {
	t.Parallel()
	root := testRoot(t)
	for _, missing := range []string{"go", "git"} {
		t.Run(missing, func(t *testing.T) {
			t.Parallel()
			s := newSuite(root, t.TempDir(), simulatedExecutor(t, root),
				func(name string) (string, error) {
					if name == missing {
						return "", ErrAcceptance
					}
					return name, nil
				})
			if s.prepare(t.Context()) == nil {
				t.Fatal("missing tool passed")
			}
		})
	}
	s := newSuite(root, t.TempDir(), func(context.Context, command) result {
		return result{err: ErrAcceptance}
	}, func(name string) (string, error) { return name, nil })
	if s.prepare(t.Context()) == nil {
		t.Fatal("failed build passed")
	}
}

func TestInstallationFailsOnBrokenSteps(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	s.execute = func(context.Context, command) result { return result{code: 1} }
	if s.install(t.Context()) == nil {
		t.Fatal("failed install passed")
	}
	if _, err := s.repository(t.Context(), "failed-repository", true); err == nil {
		t.Fatal("failed git initialization passed")
	}
}

func TestInstallationFailsOnChangedTrees(t *testing.T) {
	t.Parallel()
	root := testRoot(t)
	base := simulatedExecutor(t, root)
	s := testSuite(t)
	calls := 0
	s.execute = func(ctx context.Context, c command) result {
		got := base(ctx, c)
		if len(c.args) > 0 && c.args[0] == "agent" {
			calls++
			if calls == 2 {
				got.stdout = "+++ changed\n"
			}
		}
		return got
	}
	if s.install(t.Context()) == nil {
		t.Fatal("second install diff passed")
	}
	s = testSuite(t)
	calls = 0
	s.execute = func(ctx context.Context, c command) result {
		got := base(ctx, c)
		if len(c.args) > 0 && c.args[0] == "agent" {
			calls++
			if calls == 2 {
				checkedWrite(t, filepath.Join(environmentValue(c.env, "HOME"), "state", "extra"), "changed")
			}
		}
		return got
	}
	if s.install(t.Context()) == nil {
		t.Fatal("changed receipt tree passed")
	}
}

func TestPreservationAndDigests(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	settings := filepath.Join(root, "settings.json")
	checkedWrite(t, settings, `{"permissions":{"deny":["Read(keep-a)","Read(keep-b)"]},"enabledPlugins":{"foreign@local":true}}`)
	if err := preservedSettings(settings); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"{", `{}`, `{"permissions":{"deny":["Read(keep-b)","Read(keep-a)"]}}`} {
		checkedWrite(t, settings, body)
		if preservedSettings(settings) == nil {
			t.Fatal(body)
		}
	}
	if err := os.Remove(settings); err != nil {
		t.Fatal(err)
	}
	if preservedSettings(settings) == nil {
		t.Fatal("absent settings passed")
	}
	if _, err := treeDigests(filepath.Join(root, "absent")); err == nil {
		t.Fatal("absent tree passed")
	}
}
