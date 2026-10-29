package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/buildinfo"
	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// surfaceBinding is the production binding over an isolated home whose PATH
// holds only bin, on this machine's OS, so a fake tool at its layout path
// runs; it answers the binding, the tools directory and bin.
func surfaceBinding(t *testing.T) (binding, string, string) {
	t.Helper()
	home, bin := hometest.New(t), t.TempDir()
	b := binding{
		goos:    runtime.GOOS,
		env:     home.Getenv,
		environ: func() []string { return append(home.Environ(), "PATH="+bin) },
		home:    func() (string, error) { return home.Dir, nil },
		exe:     func() (string, error) { return filepath.Join(bin, "golaw"), nil },
		info:    buildinfo.Info{Version: testVersion, Module: "example.test/kit", Stamped: true},
	}
	return b, home.Tools(), bin
}

func surfaceApp(t *testing.T) (testApp, string, string) {
	t.Helper()
	ta := newTestApp(t)
	b, tools, bin := surfaceBinding(t)
	ta.bind = func() binding { return b }
	ta.buildInfo = func() buildinfo.Info { return b.info }
	return ta, tools, bin
}

func TestSurfaceArgsReadTheValueAndThePath(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		args        []string
		value, path string
		all, fails  bool
	}{
		"bare flag":                  {nil, surfacePresent, ".", false, false},
		"all as a positional":        {[]string{"all"}, surfacePresent, ".", true, false},
		"all as the value":           {[]string{"repo"}, surfaceAll, "repo", true, false},
		"a path":                     {[]string{"repo"}, surfacePresent, "repo", false, false},
		"an unknown value (refused)": {nil, "lenient", "", false, true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path, opts, err := surfaceArgs(tc.args, tc.value, false)
			if tc.fails {
				if !errors.Is(err, model.ErrUsage) {
					t.Errorf("surfaceArgs = %v, want ErrUsage", err)
				}
				return
			}
			if err != nil || path != tc.path || opts.all != tc.all {
				t.Errorf("surfaceArgs = %q, %+v, %v", path, opts, err)
			}
		})
	}
}

func TestSetupSurfacePrintsTheDigestWithoutInstalling(t *testing.T) {
	t.Parallel()
	ta, tools, _ := surfaceApp(t)
	if code := ta.run(t.Context(), []string{"setup", "--surface", "--print-digest"}); code != exitClean {
		t.Fatalf("exit = %d: %s", code, ta.stderr.String())
	}
	if ta.stdout.String() != toolchain.Digest()+"\n" {
		t.Errorf("stdout = %q", ta.stdout.String())
	}
	if _, found, err := toolchain.ReadReceipt(tools, "ruff"); found || err != nil {
		t.Errorf("the digest installed a tool: %t, %v", found, err)
	}
}

func TestSetupSurfaceRefusesWhatItCannotDo(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"an unknown value":           {"setup", "--surface=lenient"},
		"the digest without surface": {"setup", "--print-digest"},
		"surface with uninstall":     {"setup", "--surface", "--uninstall"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ta, _, _ := surfaceApp(t)
			if code := ta.run(t.Context(), args); code != exitFailure || !strings.Contains(ta.stderr.String(), "--") {
				t.Errorf("exit = %d, stderr %q", code, ta.stderr.String())
			}
		})
	}
}

func TestSetupSurfaceInstallsTheToolsOfTheRepositoryLanguages(t *testing.T) {
	t.Parallel()
	root := repository(t, true)
	writeFile(t, filepath.Join(root, "scripts", "a.sh"), "#!/bin/sh\necho ok\n")
	payload := []byte("#!/bin/sh\necho fake\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(payload); err != nil {
			t.Errorf("serve: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	tool := toolchain.Tool{
		Name: "fakesh", Version: "1.0.0", Distribution: toolchain.DistBinary,
		Languages: []surface.Language{surface.Shell}, Probe: []string{"--version"},
		Assets: []toolchain.Asset{{Platform: toolchain.Platform(runtime.GOOS, runtime.GOARCH), URL: server.URL + "/fakesh", SHA256: sha256Hex(payload)}},
	}
	b, tools, _ := surfaceBinding(t)
	var out bytes.Buffer
	err := b.setupSurface(t.Context(), &out, root, surfaceSetup{fetch: httpFetch, tools: []toolchain.Tool{tool}})
	if err != nil {
		t.Fatalf("setupSurface = %v", err)
	}
	path := toolchain.BinPath(tools, tool, runtime.GOOS, runtime.GOARCH)
	if !strings.Contains(out.String(), "fakesh 1.0.0 instalado e verificado em "+path) {
		t.Errorf("out = %q", out.String())
	}
}

func TestSetupSurfaceAnswersARepositoryItCannotServe(t *testing.T) {
	t.Parallel()
	b, _, _ := surfaceBinding(t)
	goOnly, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitRun(t, goOnly, "init", "-q")
	writeFile(t, filepath.Join(goOnly, "go.mod"), "module example.test/m\n\ngo 1.27\n")
	var out bytes.Buffer
	if err := b.setupSurface(t.Context(), &out, goOnly, surfaceSetup{}); err != nil || !strings.Contains(out.String(), "nada a instalar") {
		t.Errorf("no surface language = %v, out %q", err, out.String())
	}
	if err := b.setupSurface(t.Context(), &out, t.TempDir(), surfaceSetup{}); err == nil {
		t.Error("a directory outside any repository was set up")
	}
	homeless := b
	homeless.env = func(string) string { return "" }
	if err := homeless.setupSurface(t.Context(), &out, goOnly, surfaceSetup{}); !errors.Is(err, model.ErrUsage) || !strings.Contains(err.Error(), "no tools directory") {
		t.Errorf("no tools directory = %v", err)
	}
}

// The kit never installs Node: a Node below its floor on PATH refuses the
// web tools, names what PATH answered, and leaves no receipt behind.
func TestSetupSurfaceRefusesANodeBelowItsFloor(t *testing.T) {
	t.Parallel()
	ta, tools, bin := surfaceApp(t)
	root := repository(t, true)
	writeFile(t, filepath.Join(root, "web", "app.js"), "export const answer = 42;\n")
	runnertest.Install(t, bin, "node", runnertest.Behavior{Probe: []string{"--version"}, Version: "v18.0.0"})
	if code := ta.run(t.Context(), []string{"setup", "--surface", root}); code != exitFailure || !strings.Contains(ta.stderr.String(), "v18.0.0") {
		t.Errorf("exit = %d, stderr %q", code, ta.stderr.String())
	}
	if _, found, err := toolchain.ReadReceipt(tools, "eslint"); found || err != nil {
		t.Errorf("a refused runtime left a receipt: %t, %v", found, err)
	}
}

func TestHTTPFetchAnswersTheBodyOrTheFailure(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ok" {
			http.NotFound(w, r)
			return
		}
		if _, err := io.WriteString(w, "bytes"); err != nil {
			t.Errorf("serve: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	body, err := httpFetch(t.Context(), server.URL+"/ok")
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(body)
	if closeErr := body.Close(); err != nil || closeErr != nil || string(data) != "bytes" {
		t.Errorf("body = %q, %v, %v", data, err, closeErr)
	}
	if _, err := httpFetch(t.Context(), server.URL+"/absent"); !errors.Is(err, toolchain.ErrFetch) || !strings.Contains(err.Error(), "404") {
		t.Errorf("absent = %v, want ErrFetch naming the status", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := httpFetch(ctx, server.URL+"/ok"); !errors.Is(err, toolchain.ErrFetch) {
		t.Errorf("canceled = %v, want ErrFetch", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
