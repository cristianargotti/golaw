package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// ciStage opens the stages of a Dockerfile up to the ci stage, whose
// allowlist the fixture profile names (ca-certificates, git, tzdata).
const ciStage = "FROM registry.example.test/platform/golang:1.27-alpine AS base\nFROM base AS ci\n"

// TestImageInstalls pins policy.image-installs (contract D5): a package an
// install line of a listed stage names outside its allowlist, a package
// without =version under pinned_installs, and a comment line that starts
// with a forbidden prefix; the reader's declared subset is apk add and
// apt-get or apt install with plain package words, and anything else in a
// package list is a stated limit.
func TestImageInstalls(t *testing.T) {
	t.Parallel()
	const id = "policy.image-installs"
	data := profiletest.Data(t)
	red := depsProof(t, id, "red", "image")
	red.data = data
	near := depsProof(t, id, "near", "image")
	near.data = data
	at := func(content string) fixture { return depsEdit(near, "Dockerfile", ciStage+content) }
	assertDeps(t, policyfiles.ImageInstalls(), []depsCase{
		{name: "package outside the allowlist red", input: red, want: []string{"image-install-not-allowed"}},
		{name: "allowed pinned packages near", input: near},
		{name: "unpinned allowed package red", input: at("RUN apk add --no-cache git\n"), want: []string{"image-install-unpinned"}},
		{name: "unpinned package of a free stage red", input: at("FROM base AS tools\nRUN apk add make\n"), want: []string{"image-install-unpinned"}},
		{name: "inline ignore red", input: at("# lint ignore=DL3018\nRUN apk add git=2.45.2-r0\n"), want: []string{"image-inline-ignore"}},
		{name: "inline ignore spaced red", input: at("#lint   ignore=DL3018\nRUN apk add git=2.45.2-r0\n"), want: []string{"image-inline-ignore"}},
		{name: "prefix inside a RUN string near", input: at("RUN echo '# lint ignore' > /notes\n")},
		{name: "apt-get with known flags near", input: at("RUN apt-get update && apt-get install -y --no-install-recommends git=1:2.39.5-0+deb12u1\n")},
		{name: "apt-get flags first red", input: at("RUN apt-get -y install curl=7.88.1-10\n"), want: []string{"image-install-not-allowed"}},
		{name: "apt red", input: at("RUN apt install -y curl=7.88.1-10\n"), want: []string{"image-install-not-allowed"}},
		{name: "continuation red", input: at("RUN apk add --no-cache \\\n    git=2.45.2-r0 \\\n    curl=8.9.1-r0\n"), want: []string{"image-install-not-allowed"}},
		{name: "mount flag near", input: at("RUN --mount=type=cache,target=/var/cache/apk apk add git=2.45.2-r0\n")},
		{name: "chained red", input: at("RUN apk update && apk add --no-cache curl=8.9.1-r0; rm -rf /var/cache/apk\n"), want: []string{"image-install-not-allowed"}},
		{name: "exec form build near", input: at("RUN [\"go\", \"build\", \"./...\"]\n")},
		{name: "heredoc without install near", input: at("RUN <<EOF\necho ready > /etc/motd\nEOF\n")},
		{name: "unnamed stage near", input: depsEdit(near, "Dockerfile", "FROM registry.example.test/platform/golang:1.27-alpine\nRUN apk add curl=8.9.1-r0\n")},
		{name: "fixture Dockerfile near", input: depsEdit(near, "internal/x/testdata/Dockerfile", ciStage+"RUN apk add curl\n")},
		{
			name:   "over the read bound",
			input:  depsEdit(near, "Dockerfile", strings.Repeat("#", model.MaxReadBytes+1)),
			status: model.StatusUnmeasured, reason: model.ErrTooLarge.Error(),
		},
		{name: "variable", input: at("RUN apk add $PACKAGES\n"), status: model.StatusUnmeasured, reason: "Dockerfile:3:", outside: true},
		{name: "flag with an argument", input: at("RUN apk add --virtual .deps git=2.45.2-r0\n"), status: model.StatusUnmeasured, reason: "Dockerfile:3:", outside: true},
		{name: "exec form install", input: at("RUN [\"apk\", \"add\", \"curl\"]\n"), status: model.StatusUnmeasured, reason: "Dockerfile:3:", outside: true},
		{name: "heredoc install", input: at("RUN <<EOF\napk add curl\nEOF\n"), status: model.StatusUnmeasured, reason: "Dockerfile:3:", outside: true},
		{name: "unresolved stage", input: depsEdit(near, "Dockerfile", "FROM ${BASE:-alpine} AS ci\n"), status: model.StatusUnmeasured, reason: "Dockerfile:1:", outside: true},
	})
}

// TestImageInstallsAppliesOnlyWithItsData pins the gates: no profile, no
// image section, an image section without installs, pins or comments, the
// image capability off, and a tree without a root Dockerfile.
func TestImageInstallsAppliesOnlyWithItsData(t *testing.T) {
	t.Parallel()
	red := depsProof(t, "policy.image-installs", "red", "image")
	withData := func(f fixture, d *model.ProfileData) fixture { f.data = d; return f }
	noSection := *profiletest.Data(t)
	noSection.Dockerfile = nil
	stagesOnly := *profiletest.Data(t)
	stagesOnly.Dockerfile = &model.DockerfileRules{Stages: []string{"ci"}}
	noImage := withData(red, profiletest.Data(t))
	noImage.capabilities = nil
	assertDeps(t, policyfiles.ImageInstalls(), []depsCase{
		{name: "core only", input: red, status: model.StatusNotApplicable, reason: "no profile is enrolled"},
		{name: "no image section", input: withData(red, &noSection), status: model.StatusNotApplicable, reason: "the profile carries no dockerfile"},
		{name: "stages only", input: withData(red, &stagesOnly), status: model.StatusNotApplicable, reason: "the profile names no install allowlist, no pinned installs and no forbidden comment"},
		{name: "image capability off", input: noImage, status: model.StatusNotApplicable, reason: "capability image is not enabled"},
		{name: "no root Dockerfile", input: withData(depsEdit(red, "Dockerfile", ""), profiletest.Data(t))},
	})
}
