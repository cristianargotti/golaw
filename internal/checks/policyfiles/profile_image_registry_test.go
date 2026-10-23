package policyfiles_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profiletest"
)

// TestImageRegistry pins policy.image-registry (contract D5): every FROM
// reference of the root Dockerfile matches the profile's pattern, after
// the ARG defaults are substituted; scratch and a stage named before are
// never judged.
func TestImageRegistry(t *testing.T) {
	t.Parallel()
	const id = "policy.image-registry"
	data := profiletest.Data(t)
	red := depsProof(t, id, "red", "image")
	red.data = data
	near := depsProof(t, id, "near", "image")
	near.data = data
	pinned := "FROM alpine@sha256:" + strings.Repeat("a", 64) + " AS base\n"
	assertDeps(t, policyfiles.ImageRegistry(), []depsCase{
		{name: "public base red", input: red, want: []string{"image-registry"}},
		{name: "registry, stage names and scratch near", input: near},
		{name: "pinned public base red", input: depsEdit(near, "Dockerfile", pinned), want: []string{"image-registry"}},
		{
			name:  "stage named in another case near",
			input: depsEdit(near, "Dockerfile", "FROM registry.example.test/platform/golang:1.27 AS Base\nFROM BASE AS ci\nFROM Scratch\n"),
		},
		{
			name:  "stage named later red",
			input: depsEdit(near, "Dockerfile", "FROM base AS ci\nFROM registry.example.test/platform/golang:1.27 AS base\n"),
			want:  []string{"image-registry"},
		},
		{name: "fixture Dockerfile near", input: depsEdit(near, "internal/x/testdata/Dockerfile", "FROM alpine:3.22\n")},
		{name: "nested Dockerfile near", input: depsEdit(near, "deploy/local.Dockerfile", "FROM alpine:3.22\n")},
		{name: "no root Dockerfile near", input: depsEdit(near, "Dockerfile", "")},
		{
			name:   "unresolved argument",
			input:  depsEdit(near, "Dockerfile", "FROM ${UNKNOWN} AS base\n"),
			status: model.StatusUnmeasured, reason: "Dockerfile:1:", outside: true,
		},
	})
}

// TestImageRegistryAppliesOnlyWithItsSection pins the gates and the one
// pattern a validated bundle never carries.
func TestImageRegistryAppliesOnlyWithItsSection(t *testing.T) {
	t.Parallel()
	red := depsProof(t, "policy.image-registry", "red", "image")
	withData := func(f fixture, d *model.ProfileData) fixture { f.data = d; return f }
	noSection := *profiletest.Data(t)
	noSection.Registry = nil
	broken := *profiletest.Data(t)
	broken.Registry = &model.RegistryRule{Pattern: "("}
	noImage := withData(red, profiletest.Data(t))
	noImage.capabilities = nil
	assertDeps(t, policyfiles.ImageRegistry(), []depsCase{
		{name: "core only", input: red, status: model.StatusNotApplicable, reason: "no profile is enrolled"},
		{name: "no registry section", input: withData(red, &noSection), status: model.StatusNotApplicable, reason: "the profile carries no registry"},
		{name: "image capability off", input: noImage, status: model.StatusNotApplicable, reason: "capability image is not enabled"},
		{name: "pattern that does not compile", input: withData(red, &broken), status: model.StatusUnmeasured, reason: "registry.pattern"},
	})
}
