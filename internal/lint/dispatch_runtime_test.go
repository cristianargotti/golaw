package lint_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/lint"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestRunNamesTheToolWhoseRuntimeIsMissing: the runtime detail names only
// Python, so lint prefixes the tool and its pin; otherwise the person who
// reads lint --json cannot tell which check the missing Python left
// unmeasured.
func TestRunNamesTheToolWhoseRuntimeIsMissing(t *testing.T) {
	t.Parallel()
	w := newWorkspace(t, map[string]string{"infra/stack.yml": "AWSTemplateFormatVersion: '2010-09-09'\nResources: {}\n"})
	run, err := lint.Run(t.Context(), w.options(t, model.SurfaceStrict, "cfn-lint"))
	if err != nil {
		t.Fatal(err)
	}
	got := outcome(t, run, "lint.cfn-lint")
	reason := got.Result.Status.Reason
	if got.Result.Status.Kind != model.StatusUnmeasured || !strings.HasPrefix(reason, "cfn-lint ") || !strings.Contains(reason, "Python") {
		t.Errorf("status = %+v, want Unmeasured naming cfn-lint and the Python it needs", got.Result.Status)
	}
}
