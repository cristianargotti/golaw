package profile_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profile"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

func TestLoadAnswersTheSurfaceDataOfTheBundle(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	profiletest.SurfaceInstall(t, config)
	d, err := profile.Load(config, profiletest.SurfaceRef(t))
	if err != nil {
		t.Fatal(err)
	}
	if !d.Section(model.SectionAnalyzer) || !d.Section(model.SectionSonarWay) {
		t.Fatalf("sections = %+v, want analyzer and sonar-way", d)
	}
	exclusions := []string{
		"**/*_test.go", "integration/**", "**/gen/**", "**/testdata/**", "examples/*",
	}
	a := d.Analyzer
	if !slices.Equal(a.Exclusions, exclusions) || a.CoverageReport != "profile.cov" || a.Coverage != (model.CoverProfile{CoverMode: "set"}) || a.From != "" {
		t.Errorf("Analyzer = %+v", a)
	}
	want := []model.SonarWayRule{
		{Language: "js", Key: "S125", Severity: "MAJOR"},
		{Language: "js", Key: "S2681", Severity: "MAJOR"},
		{Language: "js", Key: "S3776", Severity: "CRITICAL", Params: map[string]string{"threshold": "15"}},
	}
	if d.SonarWay.Read != "2026-10-27" || len(d.SonarWay.Rules) != len(want) {
		t.Fatalf("SonarWay = %+v", d.SonarWay)
	}
	for i, rule := range d.SonarWay.Rules {
		if rule.Language != want[i].Language || rule.Key != want[i].Key || rule.Severity != want[i].Severity || rule.Params["threshold"] != want[i].Params["threshold"] {
			t.Errorf("rule %d = %+v, want %+v", i, rule, want[i])
		}
	}
}

func TestLoadOfABundleWithoutSurfaceDataLeavesThemNil(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	profiletest.Install(t, config)
	d, err := profile.Load(config, profiletest.Ref(t))
	if err != nil || d.Analyzer != nil || d.SonarWay != nil || d.Section(model.SectionAnalyzer) {
		t.Fatalf("Load = %+v, %v, want no surface data", d, err)
	}
}

func TestLoadRefusesAnAnalyzerFileChangedUnderTheStamp(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	profiletest.SurfaceInstall(t, config)
	path := filepath.Join(profile.Dir(config, profiletest.SurfaceName), profile.AnalyzerFile)
	if err := os.WriteFile(path, []byte("exclusions: [\"src/**\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Load(config, profiletest.SurfaceRef(t)); !errors.Is(err, profile.ErrDigest) {
		t.Fatalf("Load = %v, want ErrDigest: the digest covers analyzer.yml", err)
	}
}

func TestLoadRefusesAnInvalidDataFileUnderAValidStamp(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	profiletest.SurfaceInstall(t, config)
	dir := profile.Dir(config, profiletest.SurfaceName)
	if err := os.WriteFile(filepath.Join(dir, profile.AnalyzerFile), []byte("from: profile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := profile.Digest(dir)
	if err != nil {
		t.Fatal(err)
	}
	stamp := profiletest.SurfaceStamp(t, config, digest)
	ref := profiletest.SurfaceRef(t)
	ref.Digest = stamp.Digest
	if _, err := profile.Load(config, ref); !errors.Is(err, profile.ErrBundle) || !strings.Contains(err.Error(), "profile surface: analyzer.yml: ") {
		t.Fatalf("Load = %v, want ErrBundle naming analyzer.yml", err)
	}
}
