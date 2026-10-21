package cicheck_test

import (
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// advisoryFloor is the first Go release that fixes GO-2026-6604: before
// it, os.Root.Mkdir and MkdirAll can follow a junction out of the root on
// Windows, and the tree boundary of internal/runner reaches both, so the
// vuln lane refuses every older toolchain and a release built with one
// would ship the advisory.
var advisoryFloor = [3]int{1, 27, 2}

// TestWorkflowsShareOneFixedToolchain holds every setup-go step of the
// lanes to one go-version, at or above the advisory floor: a bump that
// misses one lane builds the release, or gates it, with another Go.
func TestWorkflowsShareOneFixedToolchain(t *testing.T) {
	t.Parallel()
	want := gateToolchain(t)
	for _, lane := range []string{"ci.yml", "release.yml", "vuln.yml"} {
		path := ".github/workflows/" + lane
		root, err := document(kitBytes(t, path))
		if err != nil {
			t.Fatal(err)
		}
		versions := setupGoVersions(at(root, "jobs"))
		if len(versions) == 0 {
			t.Fatalf("%s sets up no Go toolchain", path)
		}
		for _, version := range versions {
			if version != want {
				t.Fatalf("%s sets up Go %s and the release gate %s", path, version, want)
			}
		}
	}
	if !atLeast(want, advisoryFloor) {
		t.Fatalf("the lanes set up Go %s, older than the fix of GO-2026-6604", want)
	}
}

// setupGoVersions answers the go-version of each setup-go step of a jobs
// mapping, whose content alternates a job name and its body.
func setupGoVersions(jobs *yaml.Node) []string {
	var versions []string
	content := children(jobs)
	for i := 1; i < len(content); i += 2 {
		for _, step := range children(at(content[i], "steps")) {
			if strings.HasPrefix(value(at(step, "uses")), "actions/setup-go@") {
				versions = append(versions, value(at(step, "with", "go-version")))
			}
		}
	}
	return versions
}

// atLeast reports whether a version major.minor.patch is not older than
// the floor; a version that does not parse is older.
func atLeast(version string, floor [3]int) bool {
	parts := strings.Split(version, ".")
	if len(parts) != len(floor) {
		return false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return false
		}
		if n != floor[i] {
			return n > floor[i]
		}
	}
	return true
}

func TestAtLeastComparesEachPart(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		version string
		want    bool
	}{
		{"1.27.2", true},
		{"1.27.10", true},
		{"1.28.0", true},
		{"2.0.0", true},
		{"1.27.1", false},
		{"1.27.0", false},
		{"1.26.9", false},
		{"1.27", false},
		{"1.27.x", false},
	} {
		if got := atLeast(tc.version, advisoryFloor); got != tc.want {
			t.Errorf("atLeast(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}
