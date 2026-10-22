package assets_test

import (
	"bytes"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestGeneratedCIReplay(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		caps []string
	}{
		{"empty", nil},
		{"image only", []string{"image"}},
		{"go", []string{"go"}},
		{"api", []string{"api", "go"}},
		{"db", []string{"db", "go"}},
		{"image", []string{"go", "image"}},
		{"hexagonal", []string{"go", "hexagonal"}},
		{"all", model.Capabilities()},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			files := renderedRepository(t, test.caps)
			if slices.Contains(test.caps, "go") {
				files["main.go"] = []byte("package main\n\nfunc main() {}\n")
			} else {
				delete(files, "go.mod")
			}
			in := ciReplayInput{Files: files, Caps: test.caps}
			if err := checkCIReplayPlan(t, in); err != nil {
				t.Fatal(err)
			}
			if err := replayCI(t, in); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGeneratedCIPlanRejectsBrokenConnections(t *testing.T) {
	t.Parallel()
	base := renderedRepository(t, model.Capabilities())
	for _, test := range []struct {
		name, path, old, next string
		fail                  bool
	}{
		{"near", "golaw.mk", "ci:", "ci:", false},
		{"missing gate target", "golaw.mk", "ci:", "removed-ci:", true},
		{"unknown target", ciAssetPath, "make migrate-lint", "make absent-target", true},
		{
			"unused installation", ciAssetPath,
			"npm install --global squawk-cli@2.35.0",
			"npm install --global unused-tool@1.0.0",
			true,
		},
		{
			"missing installation", ciAssetPath,
			"go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0",
			"go version",
			true,
		},
		{
			"wrong capability job", ciAssetPath,
			"  db:", "  unrelated:",
			true,
		},
		{
			"unchecked download", ciAssetPath,
			"sha256sum -c -", "cat",
			true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			files := maps.Clone(base)
			original := string(files[test.path])
			changed := strings.Replace(original, test.old, test.next, 1)
			if test.old != test.next && changed == original {
				t.Fatal("mutation did not change its source")
			}
			files[test.path] = []byte(changed)
			err := checkCIReplayPlan(t, ciReplayInput{
				Files: files, Caps: model.Capabilities(),
			})
			if (err != nil) != test.fail {
				t.Fatalf("error=%v, want failure=%t", err, test.fail)
			}
		})
	}
}

func TestGeneratedCIReplayPropagatesToolFailures(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		tool string
		caps []string
	}{
		{"go", []string{"go"}},
		{"golangci-lint", []string{"go"}},
		{"gitleaks", []string{"go"}},
		{"git", []string{"go"}},
		{"curl", []string{"image"}},
		{"sha256sum", []string{"image"}},
		{"install", []string{"image"}},
		{"tar", []string{"image"}},
		{"golaw", []string{"image"}},
		{"trivy", []string{"image"}},
	} {
		t.Run(test.tool, func(t *testing.T) {
			t.Parallel()
			files := renderedRepository(t, test.caps)
			if !slices.Contains(test.caps, "go") {
				delete(files, "go.mod")
			}
			err := replayCI(t, ciReplayInput{
				Files: files, Caps: test.caps, FailTool: test.tool,
			})
			if err == nil {
				t.Fatalf("failure of %s was swallowed", test.tool)
			}
		})
	}
}

func TestGeneratedCIEnrollmentIsOptional(t *testing.T) {
	t.Parallel()
	for _, enrollment := range []string{"", strings.Repeat("a", 64)} {
		t.Run(enrollment, func(t *testing.T) {
			t.Parallel()
			in := ciReplayInput{
				Files: renderedRepository(t, []string{"go"}),
				Caps:  []string{"go"}, Enrollment: enrollment,
			}
			if err := replayCI(t, in); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGeneratedCIReplayLeavesItsInputsUntouched(t *testing.T) {
	t.Parallel()
	files := renderedRepository(t, []string{"go"})
	before := make(map[string][]byte, len(files))
	for path, data := range files {
		before[path] = bytes.Clone(data)
	}
	if err := replayCI(t, ciReplayInput{Files: files, Caps: []string{"go"}}); err != nil {
		t.Fatal(err)
	}
	for path, want := range before {
		if !bytes.Equal(files[path], want) {
			t.Fatalf("replay rewrote input %s", path)
		}
	}
}

func TestGeneratedCIStructuralReplayCoversAllValidSets(t *testing.T) {
	t.Parallel()
	for _, caps := range ciCapabilitySets() {
		files := renderedRepository(t, caps)
		if !slices.Contains(caps, "go") {
			delete(files, "go.mod")
		}
		if err := checkCIReplayPlan(t, ciReplayInput{Files: files, Caps: caps}); err != nil {
			t.Fatalf("%v: %v", caps, err)
		}
		steps, err := assets.RunSteps(files[ciAssetPath])
		if err != nil || len(steps) == 0 {
			t.Fatalf("%v: steps=%d, error=%v", caps, len(steps), err)
		}
	}
}
