package profile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// AnalyzerFile is the optional data file of M6.3.3 that describes the
// analyzer scope of a company's pipeline.
const AnalyzerFile = "analyzer.yml"

// defaultCoverMode is what go test writes without -covermode.
const defaultCoverMode = "set"

// fileCause names the profile, the data file and the cause of a refusal.
const fileCause = "profile %s: %s: %s"

// analyzerFile is analyzer.yml as the file holds it: the lists of
// surface.analyzer and the coverage profile. It carries no from key: the
// repository chooses the source, the profile only describes one.
type analyzerFile struct {
	Sources            []string         `yaml:"sources"`
	Tests              []string         `yaml:"tests"`
	TestInclusions     []string         `yaml:"test_inclusions"`
	Exclusions         []string         `yaml:"exclusions"`
	CoverageExclusions []string         `yaml:"coverage_exclusions"`
	CoverageReport     *string          `yaml:"coverage_report"`
	CoverageProfile    *coverageProfile `yaml:"coverage_profile"`
}

type coverageProfile struct {
	CoverPkg  bool   `yaml:"coverpkg"`
	CoverMode string `yaml:"covermode"`
}

// readAnalyzer reads analyzer.yml of a bundle directory, strictly; a
// bundle without one answers false.
func readAnalyzer(dir, name string) (model.AnalyzerPolicy, bool, error) {
	raw, err := os.ReadFile(filepath.Join(dir, AnalyzerFile)) //nolint:gosec // the path is an entry of the verified bundle directory
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return model.AnalyzerPolicy{}, false, nil
	case err != nil:
		return model.AnalyzerPolicy{}, false, bundlef("profile %s: "+readCause, name, AnalyzerFile, err)
	}
	var f analyzerFile
	if cause := decodeStrict(raw, &f); cause != "" {
		return model.AnalyzerPolicy{}, false, bundlef(fileCause, name, AnalyzerFile, cause)
	}
	if cause := f.invalid(); cause != "" {
		return model.AnalyzerPolicy{}, false, bundlef(fileCause, name, AnalyzerFile, cause)
	}
	return f.policy(), true, nil
}

// decodeStrict decodes one YAML document with every key known and answers
// the cause of a refusal, empty when the data decoded.
func decodeStrict(raw []byte, out any) string {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return "the file is empty"
		}
		return strings.Join(strings.Fields(err.Error()), " ")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return "a second document is not allowed"
	}
	return ""
}

// invalid answers the first entry the file may not hold: a list entry or
// the report that is not a clean relative slash path or is named twice,
// or a covermode go test does not write.
func (f analyzerFile) invalid() string {
	for _, list := range []struct {
		field   string
		entries []string
	}{
		{"sources", f.Sources},
		{"tests", f.Tests},
		{"test_inclusions", f.TestInclusions},
		{"exclusions", f.Exclusions},
		{"coverage_exclusions", f.CoverageExclusions},
	} {
		for i, entry := range list.entries {
			switch {
			case !cleanEntry(entry):
				return fmt.Sprintf("%s: %q is not a clean relative path", list.field, entry)
			case slices.Contains(list.entries[:i], entry):
				return fmt.Sprintf("%s: %q is named twice", list.field, entry)
			}
		}
	}
	if f.CoverageReport != nil && !cleanEntry(*f.CoverageReport) {
		return fmt.Sprintf("coverage_report: %q is not a clean relative path", *f.CoverageReport)
	}
	if p := f.CoverageProfile; p != nil && !slices.Contains([]string{"", defaultCoverMode, "count", "atomic"}, p.CoverMode) {
		return fmt.Sprintf("coverage_profile.covermode: %q is not set, count or atomic", p.CoverMode)
	}
	return ""
}

// cleanEntry reports a relative slash path in clean form; a trailing slash
// marks a directory, and a glob pattern is a path for this judgment.
func cleanEntry(p string) bool {
	trimmed := strings.TrimSuffix(p, "/")
	cleaned := path.Clean(trimmed)
	return p != "" && !strings.HasPrefix(p, "/") && !strings.Contains(p, `\`) && cleaned == trimmed &&
		cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

// policy converts the file into the model; the coverage mode defaults to
// set, the mode the pipeline's go test writes.
func (f analyzerFile) policy() model.AnalyzerPolicy {
	out := model.AnalyzerPolicy{
		Sources: f.Sources, Tests: f.Tests, TestInclusions: f.TestInclusions,
		Exclusions: f.Exclusions, CoverageExclusions: f.CoverageExclusions,
		Coverage: model.CoverProfile{CoverMode: defaultCoverMode},
	}
	if f.CoverageReport != nil {
		out.CoverageReport = *f.CoverageReport
	}
	if p := f.CoverageProfile; p != nil {
		out.Coverage.CoverPkg = p.CoverPkg
		if p.CoverMode != "" {
			out.Coverage.CoverMode = p.CoverMode
		}
	}
	return out
}
