package policyfiles_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// orderedStages is a Dockerfile whose named stages follow the fixture
// profile's order and end on scratch.
const orderedStages = "FROM registry.example.test/platform/certificates:1.0 AS certs\n" +
	"FROM registry.example.test/platform/golang:1.27-alpine AS base\n" +
	"FROM base AS ci\nFROM base AS builder\nFROM scratch\n"

// TestImageStages pins policy.image-stages (contract D5, section 2.2): the
// named stages of the root Dockerfile in the profile's order, each present,
// the last one from the final image; an extra stage, another Dockerfile and
// a Dockerfile under testdata are never judged.
func TestImageStages(t *testing.T) {
	t.Parallel()
	const id = "policy.image-stages"
	data := profiletest.Data(t)
	red := depsProof(t, id, "red", "image")
	red.data = data
	near := depsProof(t, id, "near", "image")
	near.data = data
	assertDeps(t, policyfiles.ImageStages(), []depsCase{
		{name: "out of order red", input: red, want: []string{"image-stage-order"}},
		{name: "extra unnamed stage near", input: near},
		{
			name:  "missing stage red",
			input: depsEdit(near, "Dockerfile", "FROM registry.example.test/platform/certificates:1.0 AS certs\nFROM registry.example.test/platform/golang:1.27-alpine AS base\nFROM base AS builder\nFROM scratch\n"),
			want:  []string{"image-stage-missing"},
		},
		{
			name:  "final image red",
			input: depsEdit(near, "Dockerfile", orderedStages+"FROM registry.example.test/platform/alpine:3.22\n"),
			want:  []string{"image-final-image"},
		},
		{
			name:  "last named stage red",
			input: depsEdit(near, "Dockerfile", "FROM registry.example.test/platform/certificates:1.0 AS certs\nFROM registry.example.test/platform/golang:1.27-alpine AS base\nFROM base AS ci\nFROM base AS builder\n"),
			want:  []string{"image-final-image"},
		},
		{
			name:  "last stage from a stage of the final image near",
			input: depsEdit(near, "Dockerfile", orderedStages+"FROM scratch AS final\nFROM final\n"),
		},
		{
			name:  "last stage from a stage of another image red",
			input: depsEdit(near, "Dockerfile", orderedStages+"FROM registry.example.test/platform/alpine:3.22 AS final\nFROM FINAL\n"),
			want:  []string{"image-final-image"},
		},
		{
			name:  "stage names in another case near",
			input: depsEdit(near, "Dockerfile", "FROM registry.example.test/platform/certificates:1.0 AS Certs\nFROM registry.example.test/platform/golang:1.27-alpine as BASE\nFROM BASE AS ci\nFROM BASE AS Builder\nFROM SCRATCH\n"),
		},
		{name: "fixture Dockerfile near", input: depsEdit(near, "internal/x/testdata/Dockerfile", "FROM scratch AS builder\nFROM alpine AS certs\n")},
		{name: "nested Dockerfile near", input: depsEdit(near, "deploy/local.Dockerfile", "FROM alpine:3.22 AS ci\n")},
		{name: "no root Dockerfile near", input: depsEdit(near, "Dockerfile", "")},
		{
			name:  "Dockerfile without a stage red",
			input: depsEdit(near, "Dockerfile", "# syntax=docker/dockerfile:1\n"),
			want:  []string{"image-stage-missing", "image-stage-missing", "image-stage-missing", "image-stage-missing"},
		},
		{
			name:   "unresolved argument",
			input:  depsEdit(near, "Dockerfile", "FROM ${BASE:-alpine} AS certs\n"),
			status: model.StatusUnmeasured, reason: "Dockerfile:1:", outside: true,
		},
	})
}

// TestImageStagesAppliesOnlyWithItsSection pins the gates of D5: no
// profile, a profile without the image section, an image section that
// names neither an order nor a final image, and the image capability off.
func TestImageStagesAppliesOnlyWithItsSection(t *testing.T) {
	t.Parallel()
	const id = "policy.image-stages"
	red := depsProof(t, id, "red", "image")
	noSection := *profiletest.Data(t)
	noSection.Dockerfile = nil
	installsOnly := *profiletest.Data(t)
	installsOnly.Dockerfile = &model.DockerfileRules{Installs: []model.InstallAllowlist{{Stage: "ci", Packages: []string{"git"}}}}
	withData := func(f fixture, d *model.ProfileData) fixture { f.data = d; return f }
	noImage := withData(red, profiletest.Data(t))
	noImage.capabilities = nil
	assertDeps(t, policyfiles.ImageStages(), []depsCase{
		{name: "core only", input: red, status: model.StatusNotApplicable, reason: "no profile is enrolled"},
		{name: "no image section", input: withData(red, &noSection), status: model.StatusNotApplicable, reason: "the profile carries no dockerfile"},
		{name: "no order and no final image", input: withData(red, &installsOnly), status: model.StatusNotApplicable, reason: "the profile names no stage order and no final image"},
		{name: "image capability off", input: noImage, status: model.StatusNotApplicable, reason: "capability image is not enabled"},
	})
}
