package policyfiles_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// localMake is a Makefile that exports the local toolchain.
const localMake = "export GOTOOLCHAIN=local\n\n.PHONY: build\n\nbuild:\n\tgo build ./...\n"

// restoredCI is an Actions file whose setup-go step is followed by the
// restore of the automatic toolchain.
const restoredCI = "on: [pull_request]\njobs:\n  ci:\n    runs-on: ubuntu-latest\n    steps:\n" +
	"      - uses: actions/checkout@08c6903cd8c0fde910a37f88322edcfb5dd907a8 # v5.0.0\n" +
	"      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0\n        with:\n          go-version-file: go.mod\n" +
	"      - name: Use the toolchain named in go.mod\n        run: echo \"GOTOOLCHAIN=auto\" >> \"$GITHUB_ENV\"\n" +
	"      - run: make ci\n"

// localDocker is a Dockerfile whose golang stage keeps the local toolchain.
const localDocker = "FROM golang:1.27 AS build\nENV GOTOOLCHAIN=local\nRUN go build ./...\n\nFROM gcr.io/distroless/static AS run\n"

type toolchainRow struct {
	name  string
	files map[string]string
	want  []string
}

func TestGotoolchainLocalJudgesTheMakefile(t *testing.T) {
	t.Parallel()
	runToolchainRows(t, []string{"go"}, []toolchainRow{
		{"the local toolchain exported", map[string]string{"Makefile": localMake}, nil},
		{"near: exported by an included file", map[string]string{"Makefile": "include golaw.mk\n", "golaw.mk": localMake}, nil},
		{"near: a default exported apart", map[string]string{"Makefile": "GOTOOLCHAIN ?= local\nexport GOTOOLCHAIN\n\nbuild:\n\tgo build ./...\n"}, nil},
		{"near: a pinned tool's target-specific switch", map[string]string{"Makefile": localMake + "deps-age: export GOTOOLCHAIN := auto\ndeps-age:\n\tgo run example.test/tool@v1.0.0\n"}, nil},
		{"near: no Makefile", map[string]string{"go.mod": "module example.test/service\n\ngo 1.27\n"}, nil},
		{"no toolchain line", map[string]string{"Makefile": "build:\n\tgo build ./...\n"}, []string{"gotoolchain-make-local"}},
		{"the automatic toolchain", map[string]string{"Makefile": "export GOTOOLCHAIN=auto\n"}, []string{"gotoolchain-make-local"}},
		{"local but not exported", map[string]string{"Makefile": "GOTOOLCHAIN = local\n"}, []string{"gotoolchain-make-local"}},
	})
}

func TestGotoolchainLocalJudgesEveryDockerfileUnderImage(t *testing.T) {
	t.Parallel()
	auto := "FROM golang:1.27\nENV GOTOOLCHAIN=auto\n"
	runToolchainRows(t, []string{"go", "image"}, []toolchainRow{
		{"a golang stage that keeps the local toolchain", map[string]string{"Dockerfile": localDocker}, nil},
		{"near: a golang stage that assigns nothing, as the golang image keeps local", map[string]string{"Dockerfile": "FROM golang:1.27 AS build\nRUN go build ./...\n"}, nil},
		{"the automatic toolchain", map[string]string{"Dockerfile": auto}, []string{"gotoolchain-docker-local"}},
		{"a pinned golang image set to auto", map[string]string{"build/Dockerfile": "FROM golang:1.27@sha256:" + strings.Repeat("a", 64) + "\nENV GOTOOLCHAIN=auto\nRUN go build ./...\n"}, []string{"gotoolchain-docker-local"}},
		{"a mirrored golang image set to auto under a variant name", map[string]string{"deploy/api.Dockerfile": "FROM mirror.example.test:5000/library/golang:1.27\nENV GOTOOLCHAIN=auto\n"}, []string{"gotoolchain-docker-local"}},
		{"an alias of a golang stage set to auto", map[string]string{"Dockerfile": "FROM golang:1.27 AS base\nFROM base AS build\nENV GOTOOLCHAIN=auto\nRUN go build ./...\n"}, []string{"gotoolchain-docker-local"}},
		{"near: an earlier build variable the last line overrides", map[string]string{"Dockerfile": "FROM golang:1.27\nENV GOTOOLCHAIN=$MODE\nENV GOTOOLCHAIN=local\n"}, nil},
		{"near: an alias named golang inherits local", map[string]string{"Dockerfile": "FROM golang:1.27 AS golang\nFROM golang AS build\nRUN go build ./...\n"}, nil},
		{"near: a stage of another image under the name golang", map[string]string{"Dockerfile": "FROM alpine:3.22 AS golang\nFROM golang\nENV GOTOOLCHAIN=auto\n"}, nil},
		{"near: a runtime stage alone", map[string]string{"Dockerfile": "FROM gcr.io/distroless/static\nCOPY service /service\n"}, nil},
		{"near: a fixture Dockerfile", map[string]string{"internal/proof/testdata/case/Dockerfile": auto}, nil},
		{"near: a file that only names a Dockerfile", map[string]string{"docs/Dockerfile.md": auto}, nil},
	})
	runToolchainRows(t, []string{"go"}, []toolchainRow{
		{"near: a golang stage set to auto without the image capability", map[string]string{"Dockerfile": auto}, nil},
	})
}

func TestGotoolchainLocalJudgesTheActionsFiles(t *testing.T) {
	t.Parallel()
	setup := "      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0\n"
	cache := "      - uses: actions/cache@0400d5f644dc74513175e3cd8d07132dd4860809 # v4.2.4\n"
	restore := "      - run: echo \"GOTOOLCHAIN=auto\" >> \"$GITHUB_ENV\"\n"
	head := "on: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n"
	tests := head + setup + "      - run: go test ./...\n"
	runToolchainRows(t, []string{"go"}, []toolchainRow{
		{"the restore after setup-go", map[string]string{".github/workflows/ci.yml": restoredCI}, nil},
		{"a test right after setup-go", map[string]string{".github/workflows/ci.yml": tests}, []string{"gotoolchain-actions-auto"}},
		{"make before the restore", map[string]string{".github/workflows/ci.yml": head + setup + "      - run: make ci\n" + restore}, []string{"gotoolchain-actions-auto"}},
		{"near: a cache step between setup-go and the restore", map[string]string{".github/workflows/ci.yml": head + setup + cache + restore + "      - run: go test ./...\n"}, nil},
		{"near: setup-go as the last step, no go after it", map[string]string{".github/workflows/release.yaml": head + setup}, nil},
		{"near: a job without setup-go", map[string]string{".github/workflows/lint.yml": head + "      - run: echo lint\n"}, nil},
		{"near: an Actions file CODEOWNERS gives to another team", map[string]string{".github/workflows/ci.yml": tests, ".github/CODEOWNERS": "* @app\n/.github/ @platform\n"}, nil},
		{"an Actions file the catch-all team owns", map[string]string{".github/workflows/ci.yml": tests, ".github/CODEOWNERS": "* @app\n/.github/ @app\n"}, []string{"gotoolchain-actions-auto"}},
	})
}

func TestGotoolchainLocalNamesTheFix(t *testing.T) {
	t.Parallel()
	res := measure(t, policyfiles.GotoolchainLocal(), fixture{files: map[string]string{
		"Makefile":                 "build:\n\tgo build ./...\n",
		"Dockerfile":               "FROM golang:1.27 AS build\nENV GOTOOLCHAIN=auto\nRUN go build ./...\n",
		".github/workflows/ci.yml": "on: [push]\njobs:\n  test:\n    steps:\n      - uses: actions/setup-go@v7\n      - run: go test ./...\n",
	}, capabilities: []string{"go", "image"}})
	want := map[string]string{
		"Makefile:0":                 "the Makefile does not export GOTOOLCHAIN=local, so a make target may run a toolchain go downloads by itself; add export GOTOOLCHAIN=local to the Makefile or a file it includes",
		"Dockerfile:2":               "the golang stage of Dockerfile sets GOTOOLCHAIN=auto, so the build may run a toolchain go downloads by itself; set ENV GOTOOLCHAIN=local or drop the line, since the golang image keeps the local toolchain",
		".github/workflows/ci.yml:5": "a step after actions/setup-go in job test of .github/workflows/ci.yml runs go before GOTOOLCHAIN=auto is restored; add the step echo \"GOTOOLCHAIN=auto\" >> \"$GITHUB_ENV\" right after actions/setup-go",
	}
	if len(res.Findings) != len(want) {
		t.Fatalf("findings = %+v", res.Findings)
	}
	for _, f := range res.Findings {
		key := fmt.Sprintf("%s:%d", f.Path, f.Line)
		if want[key] != f.Message {
			t.Errorf("%s = %q", key, f.Message)
		}
	}
	off := measure(t, policyfiles.GotoolchainLocal(), fixture{files: map[string]string{"Makefile": "build:\n"}, capabilities: []string{"cli"}})
	if off.Status.Kind != model.StatusNotApplicable {
		t.Errorf("without go = %+v", off.Status)
	}
}

// runToolchainRows measures policy.gotoolchain-local over each row under
// the capabilities.
func runToolchainRows(t *testing.T, capabilities []string, rows []toolchainRow) {
	t.Helper()
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			res := measure(t, policyfiles.GotoolchainLocal(), fixture{files: row.files, capabilities: capabilities})
			if got := codes(res); res.Status.Kind != model.StatusMeasured || (!slices.Equal(got, row.want) && len(got)+len(row.want) != 0) {
				t.Fatalf("status %+v, codes = %v, want %v (%+v)", res.Status, got, row.want, res.Findings)
			}
		})
	}
}

func TestGotoolchainLocalNamesWhereItCannotJudge(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		files map[string]string
		want  string
	}{
		"a make expansion":      {map[string]string{"Makefile": "export GOTOOLCHAIN=$(TOOLCHAIN)\n"}, "Makefile:0: GOTOOLCHAIN exported through a make expansion"},
		"a conditional export":  {map[string]string{"Makefile": "ifeq ($(OS),Windows_NT)\nexport GOTOOLCHAIN=auto\nendif\nexport GOTOOLCHAIN=local\n"}, "Makefile:2: GOTOOLCHAIN assigned inside the conditional block opened at Makefile:1"},
		"a build variable last": {map[string]string{"Dockerfile": "FROM golang:1.27\nENV GOTOOLCHAIN=$TOOLCHAIN\n"}, "Dockerfile:2: GOTOOLCHAIN set through a build variable"},
	} {
		res := measure(t, policyfiles.GotoolchainLocal(), fixture{files: tc.files, capabilities: []string{"go", "image"}})
		if !model.IsOutsideSubset(res.Status) || !strings.HasPrefix(res.Status.Reason, tc.want+": ") {
			t.Errorf("%s = %+v, want %q", name, res.Status, tc.want)
		}
	}
	big := measure(t, policyfiles.GotoolchainLocal(), fixture{files: map[string]string{"Dockerfile": strings.Repeat("#", model.MaxReadBytes+1)}, capabilities: []string{"go", "image"}})
	if big.Status.Kind != model.StatusUnmeasured || !slices.Equal(big.Unread, []string{"Dockerfile"}) {
		t.Errorf("a Dockerfile over the read bound = %+v", big)
	}
}
