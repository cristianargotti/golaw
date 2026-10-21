package policyfiles_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestMakefileCancellationPrecedesReading(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := policyfiles.Makefile().Run(ctx, model.Input{})
	if !errors.Is(err, context.Canceled) || errors.Unwrap(err) == nil ||
		!strings.Contains(err.Error(), "policy.makefile") || len(got.Findings) != 0 {
		t.Fatalf("canceled result = %+v, %v", got, err)
	}
}

type buildReadFailure struct {
	model.Snapshot

	path string
	err  error
}

func (s buildReadFailure) Read(path string) ([]byte, error) {
	if path == s.path {
		return nil, fmt.Errorf("fixture read: %w", s.err)
	}
	data, err := s.Snapshot.Read(path)
	if err != nil {
		return nil, fmt.Errorf("fixture read: %w", err)
	}
	return data, nil
}

func TestMakefileReadFailuresKeepTheActualPath(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"Makefile", "mk/checks.mk"} {
		for _, failure := range []error{model.ErrTooLarge, model.ErrUnreadable} {
			t.Run(path+"/"+failure.Error(), func(t *testing.T) {
				t.Parallel()
				policy := model.CoreDefaults()
				policy.Capabilities = []string{"go"}
				tree := proof.NewTree(map[string][]byte{
					"Makefile":     []byte("include mk/checks.mk\n"),
					"mk/checks.mk": []byte(buildTree(policy.Capabilities, "")["Makefile"]),
				})
				got, err := policyfiles.Makefile().Run(t.Context(), model.Input{
					Policy: &policy, Snapshot: buildReadFailure{Snapshot: tree, path: path, err: failure},
				})
				if err != nil || len(got.Findings) != 0 {
					t.Fatalf("failed read = %+v, %v", got, err)
				}
				requireMakefileReadFailure(t, got, path, failure)
			})
		}
	}
}

func requireMakefileReadFailure(t *testing.T, got model.Result, path string, failure error) {
	t.Helper()
	if errors.Is(failure, model.ErrTooLarge) {
		if got.Status.Kind != model.StatusMeasured || !slices.Equal(got.Unread, []string{path}) {
			t.Fatalf("bounded read = %+v", got)
		}
	} else if got.Status.Kind != model.StatusUnmeasured || len(got.Unread) != 0 ||
		!strings.Contains(got.Status.Reason, path) || !strings.Contains(got.Status.Reason, failure.Error()) {
		t.Fatalf("unreadable path = %+v", got)
	}
}

func TestMakefileMissingIncludesDoNotInventMissingTargets(t *testing.T) {
	t.Parallel()
	got := measure(t, policyfiles.Makefile(), fixture{
		capabilities: []string{"go"},
		files: map[string]string{
			"Makefile": "# header\ninclude mk/first.mk mk/second.mk\n-include optional.mk\n",
		},
	})
	if got.Status.Kind != model.StatusMeasured || !slices.Equal(codes(got), []string{"make-include-missing", "make-include-missing"}) {
		t.Fatalf("missing include results = %+v", got)
	}
	for index, name := range []string{"mk/first.mk", "mk/second.mk"} {
		finding := got.Findings[index]
		if finding.Path != "Makefile" || finding.Line != 2 || !strings.Contains(finding.Message, name) {
			t.Fatalf("include repair = %+v", finding)
		}
	}
}

func TestMakefileRecipeOverridesWithinOneFile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		line       int
	}{
		{"red two recipes", "lint:\n\tfirst\nlint:\n\tsecond\n", 4},
		{"red two inline recipes", "lint:; first\nlint:; second\n", 3},
		{"red empty earlier recipe", "lint: input\n\t\nlint:\n\tsecond\n", 4},
		{"near prerequisite-only earlier rule", "lint: input\nlint:\n\tcheck\n", 0},
		{"near prerequisite-only later rule", "lint:\n\tcheck\nlint: input\n", 0},
		{"near conditional later recipe", "lint:\n\tcheck\nifdef EXTRA\nlint:\n\tconditional\nendif\n", 0},
		{"near conditional earlier recipe", "ifdef EXTRA\nlint:\n\tconditional\nendif\nlint:\n\tcheck\n", 0},
		{"near unrelated repeated recipe", "lint:\ntool:\n\tfirst\ntool:\n\tsecond\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := buildTree([]string{"go"}, "lint")
			files["Makefile"] += tc.body
			got := measure(t, policyfiles.Makefile(), fixture{capabilities: []string{"go"}, files: files})
			requireMakefileRecipeResult(t, got, tc.line)
		})
	}
}

func requireMakefileRecipeResult(t *testing.T, got model.Result, line int) {
	t.Helper()
	if got.Status.Kind != model.StatusMeasured {
		t.Fatalf("status = %+v", got.Status)
	}
	if line == 0 {
		if len(got.Findings) != 0 {
			t.Fatalf("recipe near miss = %+v", got)
		}
		return
	}
	if !slices.Equal(codes(got), []string{"make-recipe-overridden"}) {
		t.Fatalf("recipe override = %+v", got)
	}
	finding := got.Findings[0]
	if finding.Path != "Makefile" || finding.Line != line ||
		!strings.Contains(finding.Message, "lint") || strings.Count(finding.Message, "Makefile") != 2 {
		t.Fatalf("recipe repair = %+v", finding)
	}
}

func TestMakefileConditionalIncludeAndExpressionPrecedence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, extra, want string
	}{
		{"near conditional required include", "ifdef EXTRA\ninclude missing.mk\nendif\n", "conditional include missing.mk"},
		{"near conditional optional include", "ifdef EXTRA\n-include missing.mk\nendif\n", "conditional include missing.mk"},
		{"near conditional include before eval", "ifdef EXTRA\ninclude missing.mk\nendif\n$(eval $(RULE))\n", "conditional include missing.mk"},
		{"near conditional definition before include", "ifdef EXTRA\nlint:\ninclude missing.mk\nendif\n$(eval $(RULE))\n", "conditional block opened at Makefile:2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			files := buildTree([]string{"go"}, "lint")
			files["Makefile"] += tc.extra
			got := measure(t, policyfiles.Makefile(), fixture{capabilities: []string{"go"}, files: files})
			if !model.IsOutsideSubset(got.Status) || len(got.Findings) != 0 || !strings.Contains(got.Status.Reason, tc.want) {
				t.Fatalf("conditional evidence = %+v", got)
			}
		})
	}
}

func TestMakefileLaterTargetLimitSuppressesEarlierFindings(t *testing.T) {
	t.Parallel()
	files := buildTree([]string{"go"}, "scan")
	files["Makefile"] = strings.TrimPrefix(files["Makefile"], "mod ") +
		"lint:\n\tfirst\nlint:\n\tsecond\nsc$(TARGET):\n"
	got := measure(t, policyfiles.Makefile(), fixture{capabilities: []string{"go"}, files: files})
	if !model.IsOutsideSubset(got.Status) || len(got.Findings) != 0 ||
		!strings.Contains(got.Status.Reason, "sc$(TARGET)") || !strings.Contains(got.Status.Reason, "Makefile:6:") {
		t.Fatalf("limited complete result = %+v", got)
	}
}

func TestMakefileTrailingDollarDoesNotProveMissingTarget(t *testing.T) {
	t.Parallel()
	files := buildTree([]string{"go"}, "lint")
	files["Makefile"] += "lint$ :\n"
	got := measure(t, policyfiles.Makefile(), fixture{capabilities: []string{"go"}, files: files})
	if !model.IsOutsideSubset(got.Status) || len(got.Findings) != 0 ||
		!strings.Contains(got.Status.Reason, "lint$") || !strings.Contains(got.Status.Reason, "Makefile:2:") {
		t.Fatalf("trailing dollar target = %+v", got)
	}
}
