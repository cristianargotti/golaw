package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// m2PrepareScript answers the preparation of the candidate: the tracked
// names Git lists, the go install that writes the candidate into GOBIN and
// the candidate's version, the target, in the answer the mode names.
func m2PrepareScript(t *testing.T, s *suite, mode string) *m2Script {
	t.Helper()
	versions := map[string]string{
		"valid":         "golaw 0.2.0\nbuild: source build, executable not verifiable\n",
		"wrong-version": "golaw 0.1.0\nbuild: source build, executable not verifiable\n",
		"corrupt":       "not a version report\n",
	}
	return newM2Script(t, s, func(_ context.Context, c command) (m2Reply, bool) {
		switch {
		case c.program == s.git && slices.Contains(c.args, "ls-files"):
			return m2Reply{got: result{stdout: "go.mod\x00cmd/golaw/main.go\x00"}}, true
		case c.program == s.goTool && len(c.args) > 0 && c.args[0] == "install":
			bin := environmentValue(c.env, "GOBIN")
			m2Write(t, filepath.Join(bin, executable(binaryName)), m2Executable)
			return m2Reply{}, true
		case c.program == s.binary && slices.Equal(c.args, []string{"version"}):
			if mode == "broken" {
				return m2Reply{got: result{err: ErrAcceptance}, target: true}, true
			}
			return m2Reply{got: result{stdout: versions[mode]}, target: true}, true
		}
		return m2Reply{}, false
	})
}

func m2CheckCandidateInstall(t *testing.T, s *suite, c command) {
	t.Helper()
	want := []string{"install", "github.com/cristianargotti/golaw/cmd/golaw@v0.2.0"}
	if !slices.Equal(c.args, want) || c.dir != s.root {
		t.Fatalf("install command = %+v", c)
	}
	if environmentValue(c.env, "GOBIN") != filepath.Dir(s.binary) ||
		environmentValue(c.env, "GONOSUMDB") != "github.com/cristianargotti/golaw" ||
		environmentValue(c.env, "GOSUMDB") != "sum.golang.org" {
		t.Fatalf("install environment = %q", c.env)
	}
	proxy := environmentValue(c.env, "GOPROXY")
	if !strings.HasPrefix(proxy, "file:///") ||
		!strings.HasSuffix(proxy, ",https://proxy.golang.org,direct") ||
		strings.Contains(proxy, " ") {
		t.Fatalf("proxy = %q", proxy)
	}
}

func m2PreparationFailure(t *testing.T, s *suite, mode string) executor {
	t.Helper()
	script := m2PrepareScript(t, s, "valid")
	return func(ctx context.Context, c command) result {
		switch {
		case mode == "cancel":
			return result{err: ctx.Err()}
		case mode == "enumerate" && c.program == s.git:
			return result{code: 1}
		case mode == "proxy" && c.program == s.git:
			return result{stdout: "../go.mod\x00"}
		case mode == "install" && c.program == s.goTool:
			return result{code: 2}
		}
		got := script.Execute(ctx, c)
		if c.program == s.binary && mode == "limitation" {
			got.stdout = "golaw 0.2.0\nbuild: verified\n"
		}
		if c.program == s.binary && mode == "readback" {
			if err := os.Remove(s.binary); err != nil {
				t.Fatal(err)
			}
		}
		return got
	}
}

type m2RowFailureWriter struct {
	writes int
}

func (w *m2RowFailureWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		return len(data), nil
	}
	return 0, ErrAcceptance
}
