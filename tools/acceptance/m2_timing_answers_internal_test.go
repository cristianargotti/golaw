package main

import (
	"context"
	"slices"
	"testing"
)

// m2TimingScript answers the preparation of a timing sample and its
// metadata: the tracked names of the kit, Go's version, and setup and the
// tools' probes as a lifecycle row answers them; corrupt leaves an invalid
// receipt and an unreadable Go version, broken fails setup, the target, and
// Go's version. The invocation it prepares is never run.
func m2TimingScript(t *testing.T, s *suite, mode string) *m2Script {
	t.Helper()
	return newM2Script(t, s, func(ctx context.Context, c command) (m2Reply, bool) {
		kind := m2Kind(s, c)
		switch {
		case c.program == s.git && slices.Contains(c.args, "ls-files"):
			return m2Reply{got: result{stdout: "go.mod\x00internal/source/doc.go\x00"}}, true
		case c.program == s.goTool && slices.Equal(c.args, []string{"version"}):
			return m2Reply{got: m2GoVersion(mode)}, true
		case kind == "setup" && mode == "broken":
			return m2Reply{got: result{err: ErrAcceptance}, target: true}, true
		case kind == "setup":
			got := m2Setup(ctx, t, s, c, false)
			if mode == "corrupt" {
				m2Write(t, m2ReceiptPath(t, s, c), "{")
			}
			return m2Reply{got: got, target: true}, true
		case kind == "probe":
			return m2Reply{got: m2ProbeResult(c, false)}, true
		}
		return m2Reply{}, false
	})
}

// m2GoVersion answers go version in the mode of a timing script.
func m2GoVersion(mode string) result {
	switch mode {
	case "broken":
		return result{err: ErrAcceptance}
	case "corrupt":
		return result{stdout: "not a version report\n"}
	}
	return result{stdout: "go version go1.27.0 simulated\n"}
}
