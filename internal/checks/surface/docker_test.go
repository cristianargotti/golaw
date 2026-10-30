package surface_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

// pin is a complete digest.
var pin = "sha256:" + strings.Repeat("0a", 32)

func TestDockerTagAndDigestFindsBothOnOneReference(t *testing.T) {
	t.Parallel()
	runCases(t, surface.DockerTagAndDigest(), []fileCase{
		{"a tag beside a digest", "Dockerfile", "FROM golang:1.27.3-trixie@" + pin + " AS build\n", []string{"Dockerfile:1 tag-and-digest"}},
		{"a platform flag", "Dockerfile", "FROM --platform=$BUILDPLATFORM node:22@" + pin + "\n", []string{"Dockerfile:1 tag-and-digest"}},
		{"a registry port and a tag", "Dockerfile", "FROM registry.example.test:5000/app:1.0@" + pin + "\n", []string{"Dockerfile:1 tag-and-digest"}},
		{"a lower case instruction", "deploy/valkey.Dockerfile", "from valkey/valkey:8.1@" + pin + "\n", []string{"deploy/valkey.Dockerfile:1 tag-and-digest"}},
		{"the second stage", "Dockerfile", "FROM golang@" + pin + " AS build\nFROM gcr.io/distroless/static-debian13:nonroot@" + pin + "\n", []string{"Dockerfile:2 tag-and-digest"}},
	})
	runCases(t, surface.DockerTagAndDigest(), []fileCase{
		{"a digest only", "Dockerfile", "FROM golang@" + pin + "\n", nil},
		{"a tag only", "Dockerfile", "FROM golang:1.27\n", nil},
		{"a registry port without a tag", "Dockerfile", "FROM registry.example.test:5000/app@" + pin + "\n", nil},
		{"a build argument", "Dockerfile", "ARG BASE\nFROM ${BASE}\n", nil},
		{"scratch", "Dockerfile", "FROM scratch\n", nil},
		{"a stage alias", "Dockerfile", "FROM golang@" + pin + " AS build\nFROM build\n", nil},
		{"the corrected hub shape", "Dockerfile", "# golang:1.27.3-trixie\nFROM golang@" + pin + " AS build\n", nil},
	})
}

func TestDockerTagAndDigestNamesTheFix(t *testing.T) {
	t.Parallel()
	c := surface.DockerTagAndDigest()
	src := strings.Repeat("# step\n", 48) + "FROM golang:1.27.3-trixie@" + pin + "\n"
	want := []model.Finding{{
		Path: "Dockerfile", Line: 49, Code: "tag-and-digest", Severity: model.SeverityDeny, Sonar: "docker:S8431",
		Message: "writes FROM with a tag and a digest (Sonar S8431): keep only the digest and put the tag in a comment on the line above.",
	}}
	if res := scanFiles(t, c, map[string]string{"Dockerfile": src}); !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v, want %+v", res.Findings, want)
	}
	legacy := map[string]string{"Dockerfile": "FROM golang:1.27@" + pin + "\n"}
	expectFound(t, run(t, c, writeInput(c, legacy, map[string]string{"Dockerfile": "FROM golang:1.27@" + pin + "\nUSER 65532\n"})))
}

func TestDockerFinalUserReportsARootFinalStage(t *testing.T) {
	t.Parallel()
	base := "FROM alpine@" + pin + "\n"
	runCases(t, surface.DockerFinalUser(), []fileCase{
		{"no user", "Dockerfile", base + "RUN true\n", []string{"Dockerfile:1 final-user"}},
		{"user root", "Dockerfile", base + "USER root\n", []string{"Dockerfile:2 final-user"}},
		{"user zero", "Dockerfile", base + "USER 0:0\n", []string{"Dockerfile:2 final-user"}},
		{"a user only in the build stage", "Dockerfile", "FROM golang@" + pin + " AS build\nUSER 1000\n" + base, []string{"Dockerfile:3 final-user"}},
		{"root after a user", "Dockerfile", base + "USER 65532\nUSER root\n", []string{"Dockerfile:3 final-user"}},
	})
	runCases(t, surface.DockerFinalUser(), []fileCase{
		{"a numeric user and group", "Dockerfile", base + "USER 65532:65532\n", nil},
		{"the valkey user", "deploy/valkey.Dockerfile", base + "USER 999:1000\n", nil},
		{"a named user", "Dockerfile", base + "USER app\n", nil},
		{"the last user wins", "Dockerfile", base + "USER root\nRUN apk add tini\nUSER 65532:65532\n", nil},
	})
}

func TestDockerFinalUserIsFeedback(t *testing.T) {
	t.Parallel()
	c := surface.DockerFinalUser()
	files := map[string]string{"Dockerfile": "FROM alpine@" + pin + "\nUSER root\n"}
	want := []model.Finding{{
		Path: "Dockerfile", Line: 2, Code: "final-user", Severity: model.SeverityFail, Sonar: "docker:S6471",
		Message: "the final stage runs as root through USER root (Sonar S6471): end the final stage with USER and a numeric user other than 0, such as USER 65532:65532.",
	}}
	if res := scanFiles(t, c, files); !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v, want %+v", res.Findings, want)
	}
	none := scanFiles(t, c, map[string]string{"Dockerfile": "FROM alpine@" + pin + "\n"})
	if len(none.Findings) != 1 || none.Findings[0].Message != "the final stage sets no USER, so the container runs as root (Sonar S6471): end the final stage with USER and a numeric user other than 0, such as USER 65532:65532." {
		t.Fatalf("no user = %+v", none.Findings)
	}
	if res := run(t, c, writeInput(c, nil, files)); len(res.Findings) != 1 || res.Findings[0].Severity != model.SeverityNote {
		t.Fatalf("pre write = %+v, want one note that never denies", res.Findings)
	}
}
