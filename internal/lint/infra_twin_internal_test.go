package lint

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surfacefix"
)

// twinTemplate is the CloudFormation template of both twins.
const twinTemplate = "infra/10-base.yml"

// infraTwin answers the decoded files of a twin.
func infraTwin(t *testing.T, twin surfacefix.Twin) map[string][]byte {
	t.Helper()
	files, err := surfacefix.Files(twin)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// infraCount counts the issues of one code on one path.
func infraCount(issues []Issue, path, code string) int {
	count := 0
	for _, issue := range issues {
		if issue.Path == path && Code(issue.Rule) == code {
			count++
		}
	}
	return count
}

// infraBlock answers the first and last line of the block that starts at
// the line start and ends before the line next.
func infraBlock(t *testing.T, content []byte, start, next string) (int, int) {
	t.Helper()
	lines := strings.Split(string(content), "\n")
	first := slices.Index(lines, start) + 1
	end := slices.Index(lines, next)
	if first == 0 || end < first {
		t.Fatalf("no block from %q to %q", start, next)
	}
	return first, end
}

// TestTheRedTwinYieldsTheInfraExpectations is the infrastructure half of
// M6.10 row (1): under the strict set the red twin yields each expectation
// of a lint.<tool> this part owns, at its path and count.
func TestTheRedTwinYieldsTheInfraExpectations(t *testing.T) {
	t.Parallel()
	red := infraTwin(t, surfacefix.Red)
	owned := 0
	for _, e := range surfacefix.Expectations() {
		tool, ok := strings.CutPrefix(e.Check, "lint.")
		if !ok || !slices.ContainsFunc(infraDrivers(), func(d Driver) bool { return d.Tool == tool }) {
			continue
		}
		owned++
		t.Run(e.Path+"/"+e.Code, func(t *testing.T) {
			t.Parallel()
			issues, ran := infraRun(t, infraByTool(t, infraDrivers(), tool), model.SurfaceStrict, red)
			if got := infraCount(issues, e.Path, e.Code); !ran || got != e.Count {
				t.Errorf("%s yields %s on %s %d times, want %d: %+v", tool, e.Code, e.Path, got, e.Count, issues)
			}
		})
	}
	if owned != 2 {
		t.Errorf("the red twin holds %d infrastructure expectations, want checkov and shellcheck", owned)
	}
}

// TestTheCorrectedTwinHoldsTheInfraFixes: the tools whose findings the twin
// fixed report nothing on it, and the artifact bucket now logs.
func TestTheCorrectedTwinHoldsTheInfraFixes(t *testing.T) {
	t.Parallel()
	corrected := infraTwin(t, surfacefix.Corrected)
	for _, tool := range []string{infraCfnLint, infraHadolint, infraShellcheck, infraShfmt, infraYamllint} {
		t.Run(tool, func(t *testing.T) {
			t.Parallel()
			if issues, _ := infraRun(t, infraByTool(t, infraDrivers(), tool), model.SurfaceStrict, corrected); len(issues) != 0 {
				t.Errorf("%s finds %+v in the corrected twin", tool, issues)
			}
		})
	}
	t.Run(infraCheckov, func(t *testing.T) {
		t.Parallel()
		first, last := infraBlock(t, corrected[twinTemplate], "  ArtifactBucket:", "  ArtifactBucketPolicy:")
		issues, _ := infraRun(t, infraByTool(t, infraDrivers(), infraCheckov), model.SurfaceStrict, corrected)
		for _, issue := range issues {
			if issue.Path == twinTemplate && Code(issue.Rule) == "ckv-aws-18" && issue.Line >= first && issue.Line <= last {
				t.Errorf("the artifact bucket still logs nowhere: %+v", issue)
			}
		}
	})
}
