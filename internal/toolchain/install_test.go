package toolchain_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cristianargotti/golaw/internal/hometest"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/surface"
	"github.com/cristianargotti/golaw/internal/toolchain"
)

// payload is the executable a fixture archive carries.
func payload() []byte { return []byte("#!/bin/sh\necho fake 1.0.0\n") }

// server serves fixture assets by path from a local listener and counts
// the downloads, so no test reaches the network.
type server struct {
	*httptest.Server

	hits atomic.Int64
}

func serve(t *testing.T, assets map[string][]byte) *server {
	t.Helper()
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		data, ok := assets[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write(data); err != nil {
			t.Errorf("serve %s: %v", r.URL.Path, err)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *server) fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", toolchain.ErrFetch, err)
	}
	resp, err := s.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", toolchain.ErrFetch, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Join(fmt.Errorf("%w: GET %s answered %s", toolchain.ErrFetch, url, resp.Status), resp.Body.Close())
	}
	return resp.Body, nil
}

func sum(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func targz(t *testing.T, member string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: member, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipped(t *testing.T, member string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(member)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// binaryTool is a synthetic binary tool whose one asset, for this
// platform, is served at path with the digest of asset.
func binaryTool(name, url string, asset []byte, archive, member string, lang surface.Language) toolchain.Tool {
	return toolchain.Tool{
		Name: name, Version: "1.0.0", Distribution: toolchain.DistBinary,
		Languages: []surface.Language{lang}, Probe: []string{"--version"},
		Assets: []toolchain.Asset{{Platform: toolchain.Platform(runtime.GOOS, runtime.GOARCH), URL: url, SHA256: sum(asset), Archive: archive, Member: member}},
	}
}

func options(dir string, s *server, out io.Writer, langs []surface.Language, tools ...toolchain.Tool) toolchain.InstallOptions {
	return toolchain.InstallOptions{
		Dir: dir, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Languages: langs,
		Fetch: s.fetch, Run: runner.Run, Env: []string{"PATH=" + dir}, Out: out, Tools: tools,
	}
}

// archived wraps the payload in the archive form a case names, or answers
// the bare asset.
func archived(t *testing.T, asset []byte, archive, member string) []byte {
	t.Helper()
	switch archive {
	case "tar.gz":
		return targz(t, member, payload())
	case "zip":
		return zipped(t, member, payload())
	}
	return asset
}

func assertExecutable(t *testing.T, path string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, payload()) {
		t.Fatalf("installed bytes = %q, %v", got, err)
	}
	if info, statErr := os.Stat(path); runtime.GOOS != "windows" && (statErr != nil || info.Mode()&0o100 == 0) {
		t.Errorf("installed mode = %v, %v, want executable", info, statErr)
	}
}

func TestInstallVerifiesAndPlacesEveryArchiveForm(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		asset           []byte
		archive, member string
	}{
		"bare binary": {payload(), "", ""},
		"tar.gz":      {nil, "tar.gz", "fake-1.0.0/fake"},
		"zip":         {nil, "zip", "fake.exe"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			asset := archived(t, tc.asset, tc.archive, tc.member)
			s := serve(t, map[string][]byte{"/fake": asset})
			dir := hometest.New(t).Tools()
			tool := binaryTool("fake", s.URL+"/fake", asset, tc.archive, tc.member, surface.Shell)
			var out bytes.Buffer
			receipts, err := toolchain.Install(t.Context(), options(dir, s, &out, []surface.Language{surface.Shell}, tool))
			path := toolchain.BinPath(dir, tool, runtime.GOOS, runtime.GOARCH)
			if err != nil || len(receipts) != 1 || receipts[0].Path != path || receipts[0].SHA256 != sum(payload()) {
				t.Fatalf("Install = %+v, %v", receipts, err)
			}
			assertExecutable(t, path)
			if want := "fake 1.0.0 instalado e verificado em " + path + "\n"; out.String() != want {
				t.Errorf("out = %q, want %q", out.String(), want)
			}
		})
	}
}

func TestInstallRefusesAChecksumMismatchAndWritesNothing(t *testing.T) {
	t.Parallel()
	tampered := append(bytes.Clone(payload()), '#')
	s := serve(t, map[string][]byte{"/fake": tampered})
	dir := hometest.New(t).Tools()
	tool := binaryTool("fake", s.URL+"/fake", payload(), "", "", surface.Shell)
	_, err := toolchain.Install(t.Context(), options(dir, s, io.Discard, []surface.Language{surface.Shell}, tool))
	if !errors.Is(err, toolchain.ErrChecksum) || !strings.Contains(err.Error(), sum(payload())) || !strings.Contains(err.Error(), sum(tampered)) {
		t.Fatalf("Install = %v, want ErrChecksum naming the expected and the observed digest", err)
	}
	if _, statErr := os.Stat(toolchain.BinPath(dir, tool, runtime.GOOS, runtime.GOARCH)); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a refused asset was written: %v", statErr)
	}
	if _, found, readErr := toolchain.ReadReceipt(dir, "fake"); found || readErr != nil {
		t.Errorf("a refused asset got a receipt: %t, %v", found, readErr)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	t.Parallel()
	s := serve(t, map[string][]byte{"/fake": payload()})
	dir := hometest.New(t).Tools()
	tool := binaryTool("fake", s.URL+"/fake", payload(), "", "", surface.Shell)
	if _, err := toolchain.Install(t.Context(), options(dir, s, io.Discard, []surface.Language{surface.Shell}, tool)); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	receipts, err := toolchain.Install(t.Context(), options(dir, s, &out, []surface.Language{surface.Shell}, tool))
	if err != nil || len(receipts) != 1 || s.hits.Load() != 1 {
		t.Fatalf("second Install = %+v, %v after %d downloads, want one download in all", receipts, err, s.hits.Load())
	}
	if !strings.HasPrefix(out.String(), "fake 1.0.0 já estava instalado e verificado em ") {
		t.Errorf("out = %q", out.String())
	}
}

func TestInstallSelectsTheToolsOfTheLanguagesPresent(t *testing.T) {
	t.Parallel()
	s := serve(t, map[string][]byte{"/sh": payload(), "/py": payload()})
	shell := binaryTool("shtool", s.URL+"/sh", payload(), "", "", surface.Shell)
	python := binaryTool("pytool", s.URL+"/py", payload(), "", "", surface.Python)
	only, err := toolchain.Install(t.Context(), options(hometest.New(t).Tools(), s, io.Discard, []surface.Language{surface.Python}, shell, python))
	if err != nil || len(only) != 1 || only[0].Name != "pytool" {
		t.Fatalf("python only = %+v, %v", only, err)
	}
	all := options(hometest.New(t).Tools(), s, io.Discard, nil, shell, python)
	all.All = true
	every, err := toolchain.Install(t.Context(), all)
	if err != nil || len(every) != 2 {
		t.Fatalf("--surface all = %+v, %v", every, err)
	}
}

func TestInstallNamesWhatItCannotDoAndGoesOn(t *testing.T) {
	t.Parallel()
	gz := targz(t, "x/other", payload())
	s := serve(t, map[string][]byte{"/ok": payload(), "/gz": gz})
	good := binaryTool("good", s.URL+"/ok", payload(), "", "", surface.Shell)
	elsewhere := binaryTool("elsewhere", s.URL+"/ok", payload(), "", "", surface.Shell)
	elsewhere.Assets[0].Platform = "plan9-amd64"
	wrongMember := binaryTool("member", s.URL+"/gz", gz, "tar.gz", "x/tool", surface.Shell)
	gone := binaryTool("gone", s.URL+"/absent", payload(), "", "", surface.Shell)
	receipts, err := toolchain.Install(t.Context(), options(hometest.New(t).Tools(), s, io.Discard, []surface.Language{surface.Shell}, good, elsewhere, wrongMember, gone))
	for _, want := range []error{toolchain.ErrNoAsset, toolchain.ErrArchive, toolchain.ErrFetch} {
		if !errors.Is(err, want) {
			t.Errorf("Install = %v, want %v among the failures", err, want)
		}
	}
	if len(receipts) != 1 || receipts[0].Name != "good" {
		t.Errorf("receipts = %+v, want the one tool that installs", receipts)
	}
	if !strings.Contains(err.Error(), "plan9-amd64") {
		t.Errorf("the platform refusal does not name the platform: %v", err)
	}
}

func TestInstallWithoutAToolsDirectoryIsRefused(t *testing.T) {
	t.Parallel()
	s := serve(t, nil)
	if _, err := toolchain.Install(t.Context(), options("", s, io.Discard, []surface.Language{surface.Shell})); !errors.Is(err, toolchain.ErrNoToolsDir) {
		t.Errorf("Install = %v, want ErrNoToolsDir", err)
	}
}
