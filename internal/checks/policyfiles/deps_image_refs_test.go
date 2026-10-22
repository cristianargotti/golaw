package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestImageDigestJudgesTheImagesAStageReads holds the image check to every
// external reference of a Dockerfile: COPY --from and the from of a RUN
// mount pull an image as FROM does, so a tag there is the finding, while a
// stage alias, a stage index or a digest is clean and an expansion or a
// name no stage carries is a stated limit.
func TestImageDigestJudgesTheImagesAStageReads(t *testing.T) {
	t.Parallel()
	near := depsProof(t, "policy.image-digest", "near", "image")
	pin := "alpine@sha256:" + strings.Repeat("a", 64)
	head := "FROM " + pin + " AS build\nFROM " + pin + "\n"
	code := []string{"image-digest-missing"}
	assertDeps(t, policyfiles.ImageDigest(), []depsCase{
		{name: "red COPY from a tagged image", input: depsEdit(near, "Dockerfile", head+"COPY --from=alpine:3.20 /etc/ssl /etc/ssl\n"), want: code},
		{name: "red RUN mount from a tagged image", input: depsEdit(near, "Dockerfile",
			head+"RUN --mount=type=bind,from=alpine:3.20,source=/bin,target=/mnt true\n"), want: code},
		{name: "red a later flag", input: depsEdit(near, "Dockerfile", head+"COPY --chown=1000 --from=docker.io/library/alpine /a /a\n"), want: code},
		{name: "near COPY from a stage alias", input: depsEdit(near, "Dockerfile", head+"COPY --from=build /a /a\n")},
		{name: "near COPY from a stage index", input: depsEdit(near, "Dockerfile", head+"COPY --from=0 /a /a\n")},
		{name: "near COPY from a pinned image", input: depsEdit(near, "Dockerfile", head+"COPY --from="+pin+" /a /a\n")},
		{name: "near RUN mount from a stage", input: depsEdit(near, "Dockerfile", head+"RUN --mount=type=cache,target=/root/.cache --mount=from=build,target=/x true\n")},
		{name: "near COPY without from", input: depsEdit(near, "Dockerfile", head+"COPY --chown=1000 go.mod ./\n")},
		{
			name: "limit an expansion", input: depsEdit(near, "Dockerfile", head+"COPY --from=${BASE} /a /a\n"),
			status: model.StatusUnmeasured, outside: true, reason: "Dockerfile:3: COPY --from=${BASE}",
		},
		{
			name: "limit a name no stage carries", input: depsEdit(near, "Dockerfile", head+"COPY --from=certs /a /a\n"),
			status: model.StatusUnmeasured, outside: true, reason: "Dockerfile:3: COPY --from=certs",
		},
	})
}

// TestGotoolchainLocalReadsNoImageReference keeps a reference only the image
// check judges out of the toolchain check's answer.
func TestGotoolchainLocalReadsNoImageReference(t *testing.T) {
	t.Parallel()
	const dockerfile = "FROM golang:1.27 AS build\nENV GOTOOLCHAIN=local\nRUN go build ./...\n"
	plain := depsEdit(depsProof(t, "policy.gotoolchain-local", "near", "go", "image"), "Dockerfile", dockerfile)
	want := measure(t, policyfiles.GotoolchainLocal(), plain)
	expanded := depsEdit(plain, "Dockerfile", dockerfile+"COPY --from=${BASE} /a /a\n")
	if got := measure(t, policyfiles.GotoolchainLocal(), expanded); got.Status.Kind != want.Status.Kind || len(got.Findings) != len(want.Findings) || want.Status.Kind != model.StatusMeasured {
		t.Fatalf("beside an expanded COPY --from = %+v, without it %+v", got, want)
	}
}
