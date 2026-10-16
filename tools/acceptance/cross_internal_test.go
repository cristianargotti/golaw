package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// crossExecutor answers each cross case with the answer its expectation
// names, and refuses a hook process that runs outside the session the
// payload names or a target that is not an absolute path.
func crossExecutor(t *testing.T, s *suite, wants map[string]string) executor {
	t.Helper()
	base := s.execute
	return func(ctx context.Context, c command) result {
		if len(c.args) == 0 || c.args[0] != commandHook {
			return base(ctx, c)
		}
		var envelope struct {
			Use   string `json:"tool_use_id"`
			Cwd   string `json:"cwd"`
			Input struct {
				Path string `json:"file_path"`
			} `json:"tool_input"`
		}
		if err := json.Unmarshal([]byte(c.input), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Cwd != c.dir || !filepath.IsAbs(envelope.Input.Path) {
			return result{err: ErrAcceptance}
		}
		return simulatedAnswer(wants[envelope.Use])
	}
}

func TestCrossRows(t *testing.T) {
	t.Parallel()
	s := testSuite(t)
	repo, session := filepath.Join(s.base, "repo"), filepath.Join(s.base, "session")
	wants := map[string]string{}
	cases := s.crossCases(repo, session)
	for i := range cases {
		c := &cases[i]
		wants[c.f.name] = c.f.want
		target, ok := c.f.input[keyFilePath].(string)
		if !ok || (c.dir == session) != strings.HasPrefix(target, repo) {
			t.Fatalf("%s runs in %s over %v", c.f.name, c.dir, c.f.input[keyFilePath])
		}
	}
	s.execute = crossExecutor(t, s, wants)
	rows := s.crossRows(t.Context())
	if len(rows) != len(wants) {
		t.Fatalf("rows = %+v", rows)
	}
	for _, item := range rows {
		if item.category != categoryPolicy || wants[item.name] == "" {
			t.Fatalf("%+v", item)
		}
		assertPass(t, item)
	}
	inverted := map[string]string{}
	for name, want := range wants {
		inverted[name] = map[string]string{wantDeny: wantAllow, wantAllow: wantDeny}[want]
	}
	s.execute = crossExecutor(t, s, inverted)
	allFail(t, "an inverted answer", s.crossRows(t.Context()))
	s.execute = func(context.Context, command) result { return result{err: ErrAcceptance} }
	allFail(t, "a failed setup", s.crossRows(t.Context()))
}

func allFail(t *testing.T, what string, rows []row) {
	t.Helper()
	for _, item := range rows {
		if item.status != statusFail {
			t.Fatalf("%s passed: %+v", what, item)
		}
	}
}
