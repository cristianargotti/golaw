package buildinfo_test

import (
	"runtime/debug"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/buildinfo"
)

func TestReadWithoutStampIsDevel(t *testing.T) {
	t.Parallel()
	cases := map[string]*debug.BuildInfo{
		"nil":         nil,
		"unversioned": {Main: debug.Module{Version: "(devel)"}},
	}
	for name, info := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := buildinfo.Read(info, info != nil)
			if got.Version != buildinfo.DevelVersion {
				t.Fatalf("version = %q, want %q", got.Version, buildinfo.DevelVersion)
			}
			if got.Stamped != (info != nil) {
				t.Fatalf("stamped = %v", got.Stamped)
			}
		})
	}
}

func TestReadStripsTheVAndKeepsTheRevision(t *testing.T) {
	t.Parallel()
	info := &debug.BuildInfo{
		Main: debug.Module{Path: "github.com/cristianargotti/golaw", Version: "v0.1.0", Sum: "h1:abc"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "5ca8e5a"},
			{Key: "vcs.modified", Value: "true"},
			{Key: "GOOS", Value: "linux"},
		},
	}
	got := buildinfo.Read(info, true)
	want := buildinfo.Info{
		Version: "0.1.0", Module: "github.com/cristianargotti/golaw", Sum: "h1:abc",
		Revision: "5ca8e5a", Modified: true, Stamped: true,
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReleaseIsTheExactVersionOfAStampedBinary(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		info    buildinfo.Info
		release string
		ok      bool
	}{
		"release":        {buildinfo.Info{Version: "0.1.0", Stamped: true}, "0.1.0", true},
		"pseudo version": {buildinfo.Info{Version: "0.0.0-20261015004959-dd849024b474", Stamped: true}, "", false},
		"development":    {buildinfo.Info{Version: buildinfo.DevelVersion, Stamped: true}, "", false},
		"unstamped":      {buildinfo.Info{Version: "0.1.0"}, "", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			release, ok := tc.info.Release()
			if release != tc.release || ok != tc.ok {
				t.Fatalf("release = %q, %v, want %q, %v", release, ok, tc.release, tc.ok)
			}
		})
	}
}

func TestLinesEndWithTheSourceBuildState(t *testing.T) {
	t.Parallel()
	full := buildinfo.Info{Version: "0.1.0", Module: "m", Sum: "h1:x", Revision: "abc", Modified: true}
	want := []string{
		"golaw 0.1.0", "module: m", "sum: h1:x", "revision: abc (modified)",
		"build: source build, executable not verifiable",
	}
	if got := full.Lines(); !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	lane := buildinfo.Info{Version: "0.5.0", Module: "m", Revision: "abc", Stamped: true}
	if got := lane.Lines(); got[len(got)-1] != "build: release version, executable verified only by golaw doctor --release <dir>" {
		t.Fatalf("release lane lines = %q", got)
	}
	unstamped := buildinfo.Info{Version: "0.5.0"}
	if got := unstamped.Lines(); got[len(got)-1] != "build: source build, executable not verifiable" {
		t.Fatalf("unstamped lines = %q", got)
	}
	bare := buildinfo.Info{Version: buildinfo.DevelVersion}
	if got := bare.Lines(); len(got) != 2 || got[0] != "golaw 0.0.0-devel" {
		t.Fatalf("bare lines = %q", got)
	}
}

func TestCurrentIsStampedUnderGoTest(t *testing.T) {
	t.Parallel()
	got := buildinfo.Current()
	if !got.Stamped || got.Module != "github.com/cristianargotti/golaw" {
		t.Fatalf("current = %+v", got)
	}
}
