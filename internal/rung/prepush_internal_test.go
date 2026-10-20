package rung

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/runner"
)

func TestPrePushLetsAScannerThatCannotStartGo(t *testing.T) {
	t.Parallel()
	missing := fmt.Errorf("%w: govulncheck: executable file not found in PATH", runner.ErrStart)
	out := prepush{}.read("0123456789ab", runner.Output{}, missing)
	if out.Code != codeClean || len(out.Lines) != 1 || !strings.Contains(out.Lines[0], "govulncheck could not judge it, the pipeline judges it") || !strings.HasSuffix(out.Lines[0], "install govulncheck 1.7.0 with golaw setup") {
		t.Errorf("a scanner PATH lacks = %+v", out)
	}
	if fixedIn("") != "no version yet" || firstLine([]byte("\nfirst\nsecond\n")) != "first" {
		t.Error("the helpers of the lines")
	}
}
