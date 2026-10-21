package policyfiles_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

func TestImageDigest(t *testing.T) {
	t.Parallel()
	const id = "policy.image-digest"
	const code = "image-digest-missing"
	red := depsProof(t, id, "red", "image")
	near := depsProof(t, id, "near", "image")
	pin := "alpine@sha256:" + strings.Repeat("a", 64)
	listed := depsEdit(red, "Dockerfile", "FROM registry.example.test/platform/certs:latest\nFROM alpine:3.22\n")
	listed.options = model.Options{"allowed_tags": model.StringsValue("registry.example.test/platform/*")}
	unlisted := listed
	unlisted.options = model.Options{"allowed_tags": model.StringsValue("registry.example.test/other/*")}
	assertDeps(t, policyfiles.ImageDigest(), []depsCase{
		{name: "tag red", input: red, want: []string{code}},
		{name: "arg and stage alias near", input: near},
		{
			name:  "fixture Dockerfile near",
			input: depsEdit(near, "internal/x/testdata/image/Dockerfile", "FROM alpine:3.22\n"),
		},
		{name: "scratch near", input: depsEdit(red, "Dockerfile", "FROM scratch\n")},
		{
			name: "tag with digest near",
			input: depsEdit(red, "Dockerfile",
				"FROM alpine:3.22@sha256:"+strings.Repeat("b", 64)+"\n"),
		},
		{
			name:  "short digest red",
			input: depsEdit(red, "Dockerfile", "FROM alpine@sha256:abcd\n"),
			want:  []string{code},
		},
		{
			name:  "second external image red",
			input: depsEdit(red, "Dockerfile", "FROM "+pin+" AS build\nFROM alpine:3.22\n"),
			want:  []string{code},
		},
		{
			name:  "literal platform near",
			input: depsEdit(red, "Dockerfile", "FROM --platform=linux/amd64 "+pin+"\n"),
		},
		{
			name:  "nested Dockerfile red",
			input: depsEdit(near, "deploy/Dockerfile", "FROM alpine:3.22\n"),
			want:  []string{code},
		},
		{
			name:  "variant Dockerfile red",
			input: depsEdit(near, "Dockerfile.dev", "FROM alpine:3.22\n"),
			want:  []string{code},
		},
		{
			name:  "suffixed Dockerfile red",
			input: depsEdit(near, "deploy/api.Dockerfile", "FROM alpine:3.22\n"),
			want:  []string{code},
		},
		{name: "page that names a Dockerfile near", input: depsEdit(near, "docs/Dockerfile.md", "FROM alpine:3.22\n")},
		{name: "ignore file beside a Dockerfile near", input: depsEdit(near, "Dockerfile.dockerignore", "FROM alpine:3.22\n")},
		{
			name:   "arg without default",
			input:  depsEdit(red, "Dockerfile", "ARG BASE\nFROM ${BASE}\n"),
			status: model.StatusUnmeasured, reason: "Dockerfile:2:", outside: true,
		},
		{
			name:   "parameter expression",
			input:  depsEdit(red, "Dockerfile", "FROM ${BASE:-alpine}\n"),
			status: model.StatusUnmeasured, reason: "Dockerfile:1:", outside: true,
		},
		{
			name: "heredoc text is not a stage",
			input: depsEdit(red, "Dockerfile",
				"FROM "+pin+"\nRUN <<EOF\nFROM alpine:3.22\nEOF\n"),
		},
		{
			name: "automatic platform argument near",
			input: depsEdit(red, "Dockerfile",
				"FROM --platform=$BUILDPLATFORM "+pin+" AS build\nFROM --platform=${TARGETPLATFORM} "+pin+"\n"),
		},
		{
			name:  "automatic platform argument red",
			input: depsEdit(red, "Dockerfile", "FROM --platform=$BUILDPLATFORM golang:1.27 AS build\n"),
			want:  []string{code},
		},
		{
			name:  "converted line endings near",
			input: depsEdit(red, "Dockerfile", "FROM "+pin+" AS build\r\nENV GOTOOLCHAIN=local\r\nFROM build\r\n"),
		},
		{name: "listed registry left, the rest found", input: listed, want: []string{code}},
		{name: "unlisted registry red", input: unlisted, want: []string{code, code}},
		{name: "no image file", input: fixture{capabilities: []string{"image"}, files: map[string]string{}}},
		{
			name: "capability off", input: depsProof(t, id, "red", "go"),
			status: model.StatusNotApplicable, reason: "capability image",
		},
	})
}

func TestImageDigestOptionSchema(t *testing.T) {
	t.Parallel()
	definition := policyfiles.ImageDigest().Spec()
	if len(definition.Options) != 1 {
		t.Fatalf("options = %+v, want one tag allowlist", definition.Options)
	}
	option := definition.Options[0]
	if option.Name != "allowed_tags" || option.Kind != model.OptionStrings ||
		option.Tighten != model.TightenShrink || option.Default.Kind != model.OptionStrings ||
		len(option.Default.Strings) != 0 {
		t.Fatalf("option = %+v, want a shrinking empty string list", option)
	}
}

func TestImageDigestAllowedTagPatterns(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, reference, pattern string
		finding                  bool
	}{
		{"registry port and tag", "registry.example.test:5000/platform/base:latest", "registry.example.test:5000/platform/*", false},
		{"registry port without tag", "registry.example.test:5000/platform/base", "registry.example.test:5000/platform/*", false},
		{"port must match", "registry.example.test:5000/platform/base:latest", "registry.example.test/platform/*", true},
		{"exact repository ignores tag", "alpine:3.22", "alpine", false},
		{"digest is removed before matching", "alpine:3.22@sha256:abcd", "alpine", false},
		{"glob does not cross slash", "registry.example.test/platform/nested/base:latest", "registry.example.test/platform/*", true},
	} {
		input := fixture{
			files:        map[string]string{"Dockerfile": "FROM " + row.reference + "\n"},
			capabilities: []string{"image"}, options: model.Options{"allowed_tags": model.StringsValue(row.pattern)},
		}
		var want []string
		if row.finding {
			want = []string{"image-digest-missing"}
		}
		assertDeps(t, policyfiles.ImageDigest(), []depsCase{{name: row.name, input: input, want: want}})
	}
	invalid := depsProof(t, "policy.image-digest", "red", "image")
	invalid.options = model.Options{"allowed_tags": model.StringsValue("[")}
	assertDeps(t, policyfiles.ImageDigest(), []depsCase{{
		name: "invalid pattern is not a finding", input: invalid,
		status: model.StatusUnmeasured, reason: "allowed_tags",
	}})
}

func TestImageDigestScopeAndLocations(t *testing.T) {
	t.Parallel()
	input := fixture{capabilities: []string{"image"}, files: map[string]string{
		"Dockerfile":                         "# build image\nFROM alpine:3.22\n",
		"deploy/Dockerfile":                  "FROM busybox:latest\n",
		"internal/x/testdata/Dockerfile":     "FROM alpine:3.22\n",
		"docs/Dockerfile.txt":                "FROM alpine:3.22\n",
		"deploy/Dockerfile.dev.dockerignore": "FROM alpine:3.22\n",
	}}
	got := measure(t, policyfiles.ImageDigest(), input)
	if got.Status.Kind != model.StatusMeasured || got.Scope != "2 Dockerfiles" || len(got.Findings) != 2 {
		t.Fatalf("result = %+v, want two measured Dockerfiles", got)
	}
	for index, location := range []struct {
		path string
		line int
	}{{"Dockerfile", 2}, {"deploy/Dockerfile", 1}} {
		finding := got.Findings[index]
		if finding.Path != location.path || finding.Line != location.line ||
			!strings.Contains(finding.Message, "sha256") {
			t.Fatalf("finding = %+v, want location %+v and a digest repair", finding, location)
		}
	}
	empty := measure(t, policyfiles.ImageDigest(), fixture{capabilities: []string{"image"}})
	if empty.Scope != "0 Dockerfiles" {
		t.Fatalf("empty scope = %q", empty.Scope)
	}
}

func TestImageDigestAliasesAndReadLimits(t *testing.T) {
	t.Parallel()
	red := depsProof(t, "policy.image-digest", "red", "image")
	assertDeps(t, policyfiles.ImageDigest(), []depsCase{
		{
			name: "alias cannot excuse its own base", want: []string{"image-digest-missing"},
			input: depsEdit(red, "Dockerfile", "FROM alpine AS alpine\nFROM alpine\n"),
		},
		{
			name: "later alias is not an earlier base", want: []string{"image-digest-missing"},
			input: depsEdit(red, "Dockerfile", "FROM build\nFROM scratch AS build\n"),
		},
		{
			name: "later limit discards earlier findings", status: model.StatusUnmeasured, outside: true,
			input: depsEdit(red, "deploy/Dockerfile", "FROM ${UNKNOWN}\n"), reason: "deploy/Dockerfile:1:",
		},
		{
			name: "read bound discards earlier findings", status: model.StatusUnmeasured,
			input:  depsEdit(red, "deploy/Dockerfile", strings.Repeat("x", model.MaxReadBytes+1)),
			reason: model.ErrTooLarge.Error(),
		},
	})
}

func TestImageDigestSnapshotReadErrors(t *testing.T) {
	t.Parallel()
	for _, failure := range []error{model.ErrNotInSnapshot, model.ErrUnreadable} {
		t.Run(failure.Error(), func(t *testing.T) {
			t.Parallel()
			tree := imageReadFailureSnapshot{
				Tree: proof.NewTree(map[string][]byte{"Dockerfile": []byte("FROM scratch\n")}),
				err:  fmt.Errorf("Dockerfile: %w", failure),
			}
			got, err := policyfiles.ImageDigest().Run(t.Context(), model.Input{Snapshot: tree})
			if err != nil || got.Status.Kind != model.StatusUnmeasured || len(got.Findings) != 0 ||
				!strings.Contains(got.Status.Reason, "Dockerfile") {
				t.Fatalf("result = %+v, error = %v, want named read failure", got, err)
			}
		})
	}
}

type imageReadFailureSnapshot struct {
	*proof.Tree

	err error
}

func (s imageReadFailureSnapshot) Read(_ string) ([]byte, error) { return nil, s.err }

func TestImageDigestRepairMessage(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ file, image string }{
		{"Dockerfile", "alpine:3.22"},
		{"deploy/Dockerfile", "busybox:latest"},
	} {
		t.Run(row.file, func(t *testing.T) {
			t.Parallel()
			input := fixture{capabilities: []string{"image"}, files: map[string]string{
				row.file: "# base image\nFROM " + row.image + "\n",
			}}
			got := measure(t, policyfiles.ImageDigest(), input)
			if len(got.Findings) != 1 {
				t.Fatalf("findings = %+v, want one missing digest", got.Findings)
			}
			want := "pin the base image " + row.image + " in " + row.file +
				" to a complete digest: write the reference with @sha256: and its 64 hexadecimal digits"
			if got.Findings[0].Message != want {
				t.Fatalf("message = %q, want %q", got.Findings[0].Message, want)
			}
		})
	}
}

func TestImageDigestBuildKitInputs(t *testing.T) {
	t.Parallel()
	red := depsProof(t, "policy.image-digest", "red", "image")
	pin := "alpine@sha256:" + strings.Repeat("a", 64)
	assertDeps(t, policyfiles.ImageDigest(), []depsCase{
		{
			name:  "byte order mark preserves first stage alias",
			input: depsEdit(red, "Dockerfile", "\ufeffFROM "+pin+" AS build\r\nFROM build\r"),
		},
		{
			name:  "multiple global argument defaults",
			input: depsEdit(red, "Dockerfile", "ARG REGISTRY=example.test BASE="+pin+"\nFROM $BASE\n"),
		},
		{
			name:  "unused multiple global argument names",
			input: depsEdit(red, "Dockerfile", "ARG A B\nFROM "+pin+"\n"),
		},
	})
}
