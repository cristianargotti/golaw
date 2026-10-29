package toolchain_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runnertest"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// npmBin is a PATH with node at its floor and a fake npm answering b.
func npmBin(t *testing.T, b runnertest.Behavior) string {
	t.Helper()
	bin := t.TempDir()
	runtimeOnPath(t, bin, "node", "v24.21.0")
	runnertest.Install(t, bin, "npm", b)
	return bin
}

func npmOptions(t *testing.T, dir, bin string, out io.Writer) toolchain.InstallOptions {
	t.Helper()
	o := options(dir, serve(t, nil), out, []surface.Language{surface.JavaScript}, npmTool())
	o.Env = []string{"PATH=" + bin}
	return o
}

// pipAt installs a fake pip in the kit's venv answering b.
func pipAt(t *testing.T, dir string, b runnertest.Behavior) string {
	t.Helper()
	pip := toolchain.ExecPath(dir, toolchain.Tool{Name: "pip", Distribution: toolchain.DistPyPI, Bin: "pip"}, runtime.GOOS, runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(pip), 0o700); err != nil {
		t.Fatal(err)
	}
	runnertest.Install(t, filepath.Dir(pip), "pip", b)
	return filepath.Dir(pip)
}

func pypiOptions(t *testing.T, dir, bin string) toolchain.InstallOptions {
	t.Helper()
	o := options(dir, serve(t, nil), io.Discard, []surface.Language{surface.YAML}, pypiTool())
	o.Env = []string{"PATH=" + bin}
	return o
}

func TestInstallKeepsAVerifiedNpmGroupAndStartsNoPackageManager(t *testing.T) {
	t.Parallel()
	dir := hometest.New(t).Tools()
	bin := npmBin(t, runnertest.Behavior{})
	placed(t, dir, npmTool())
	if _, err := toolchain.Install(t.Context(), npmOptions(t, dir, bin, io.Discard)); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	receipts, err := toolchain.Install(t.Context(), npmOptions(t, dir, bin, &out))
	if err != nil || len(receipts) != 1 || len(runnertest.Calls(t, bin, "npm")) != 1 {
		t.Fatalf("second Install = %+v, %v, npm calls %v", receipts, err, runnertest.Calls(t, bin, "npm"))
	}
	if !strings.HasPrefix(out.String(), "eslint 10.13.0 já estava instalado e verificado em ") {
		t.Errorf("out = %q", out.String())
	}
}

func TestInstallNamesThePackageManagerStepThatFailed(t *testing.T) {
	t.Parallel()
	dir := hometest.New(t).Tools()
	bin := npmBin(t, runnertest.Behavior{Code: 1, Stderr: "npm error ERESOLVE could not resolve\nmore\n"})
	_, err := toolchain.Install(t.Context(), npmOptions(t, dir, bin, io.Discard))
	if !errors.Is(err, toolchain.ErrInstall) || !strings.Contains(err.Error(), "ci exited 1: npm error ERESOLVE could not resolve") {
		t.Fatalf("Install = %v, want ErrInstall with the exit and the first stderr line", err)
	}
	if _, found, readErr := toolchain.ReadReceipt(dir, "eslint"); found || readErr != nil {
		t.Errorf("a failed npm ci left a receipt: %t, %v", found, readErr)
	}
}

func TestInstallRefusesAToolThePackageManagerDidNotPlace(t *testing.T) {
	t.Parallel()
	dir := hometest.New(t).Tools()
	bin := npmBin(t, runnertest.Behavior{})
	receipts, err := toolchain.Install(t.Context(), npmOptions(t, dir, bin, io.Discard))
	if !errors.Is(err, toolchain.ErrInstall) || len(receipts) != 0 || !strings.Contains(err.Error(), "left nothing at") {
		t.Fatalf("Install = %+v, %v, want ErrInstall naming the empty path", receipts, err)
	}
}

func TestInstallWantsNpmBesideNode(t *testing.T) {
	t.Parallel()
	dir, bin := hometest.New(t).Tools(), t.TempDir()
	runtimeOnPath(t, bin, "node", "v24.21.0")
	_, err := toolchain.Install(t.Context(), npmOptions(t, dir, bin, io.Discard))
	if !errors.Is(err, toolchain.ErrRuntime) || !strings.Contains(err.Error(), "npm is not on PATH beside node") {
		t.Fatalf("Install = %v, want ErrRuntime naming npm", err)
	}
}

func TestInstallRefusesThePyPIToolsWithoutPython(t *testing.T) {
	t.Parallel()
	dir := hometest.New(t).Tools()
	pip := pipAt(t, dir, runnertest.Behavior{})
	_, err := toolchain.Install(t.Context(), pypiOptions(t, dir, t.TempDir()))
	if !errors.Is(err, toolchain.ErrRuntime) || !strings.Contains(err.Error(), "Python 3.10 or newer") {
		t.Fatalf("Install = %v, want ErrRuntime with the bounds", err)
	}
	if calls := runnertest.Calls(t, pip, "pip"); len(calls) != 0 {
		t.Errorf("pip ran without python: %v", calls)
	}
}

func TestInstallReusesTheVenvAndNamesAFailedPip(t *testing.T) {
	t.Parallel()
	dir, bin := hometest.New(t).Tools(), t.TempDir()
	runtimeOnPath(t, bin, pythonName(), "Python 3.12.1")
	placed(t, dir, toolchain.Tool{Name: "python", Distribution: toolchain.DistPyPI, Bin: "python"})
	pipAt(t, dir, runnertest.Behavior{Code: 1, Stderr: "ERROR: Hashes are required\n"})
	_, err := toolchain.Install(t.Context(), pypiOptions(t, dir, bin))
	if !errors.Is(err, toolchain.ErrInstall) || !strings.Contains(err.Error(), "install exited 1: ERROR: Hashes are required") {
		t.Fatalf("Install = %v, want ErrInstall with the pip line", err)
	}
	if calls := runnertest.Calls(t, bin, pythonName()); len(calls) != 1 || !slices.Equal(calls[0], []string{"--version"}) {
		t.Errorf("python calls = %v, want only the probe beside an existing venv", calls)
	}
}

func TestInstallRefusesAnArchiveWithoutItsMember(t *testing.T) {
	t.Parallel()
	notGzip := []byte("not a gzip stream")
	tests := map[string]struct {
		asset   []byte
		archive string
		want    string
	}{
		"broken tar.gz":      {notGzip, "tar.gz", "gzip"},
		"broken zip":         {notGzip, "zip", "zip"},
		"zip without member": {zipped(t, "other.exe", payload()), "zip", "has no member fake.exe"},
		"tar.xz":             {notGzip, "tar.xz", "the installer cannot open"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := serve(t, map[string][]byte{"/fake": tc.asset})
			tool := binaryTool("fake", s.URL+"/fake", tc.asset, tc.archive, "fake.exe", surface.Shell)
			_, err := toolchain.Install(t.Context(), options(hometest.New(t).Tools(), s, io.Discard, []surface.Language{surface.Shell}, tool))
			if !errors.Is(err, toolchain.ErrArchive) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Install = %v, want ErrArchive naming %q", err, tc.want)
			}
		})
	}
}

func TestInstallWithoutAFetcherIsAFetchError(t *testing.T) {
	t.Parallel()
	s := serve(t, nil)
	tool := binaryTool("fake", s.URL+"/fake", payload(), "", "", surface.Shell)
	o := options(hometest.New(t).Tools(), s, io.Discard, []surface.Language{surface.Shell}, tool)
	o.Fetch, o.Run, o.Out = nil, nil, nil
	if _, err := toolchain.Install(t.Context(), o); !errors.Is(err, toolchain.ErrFetch) {
		t.Errorf("Install = %v, want ErrFetch", err)
	}
}

func TestInstallReadsTheTableAndSelectsNothingForNoLanguage(t *testing.T) {
	t.Parallel()
	s := serve(t, nil)
	o := options(hometest.New(t).Tools(), s, io.Discard, nil)
	o.Tools = nil
	receipts, err := toolchain.Install(t.Context(), o)
	if err != nil || len(receipts) != 0 || s.hits.Load() != 0 {
		t.Errorf("Install = %+v, %v after %d downloads, want nothing", receipts, err, s.hits.Load())
	}
}

// refusing is an output the person can no longer read.
type refusing struct{}

func (refusing) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestInstallReportsAnOutputThatRefusesTheLine(t *testing.T) {
	t.Parallel()
	s := serve(t, map[string][]byte{"/fake": payload()})
	tool := binaryTool("fake", s.URL+"/fake", payload(), "", "", surface.Shell)
	_, err := toolchain.Install(t.Context(), options(hometest.New(t).Tools(), s, refusing{}, []surface.Language{surface.Shell}, tool))
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Errorf("Install = %v, want the write error", err)
	}
}
