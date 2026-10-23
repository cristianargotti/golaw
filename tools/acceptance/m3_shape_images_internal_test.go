package main

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

func TestM3PlatformImagesMayStayByTagOnlyWhenListed(t *testing.T) {
	t.Parallel()
	capabilities := []string{"go", "image"}
	listed := model.Options{"allowed_tags": model.StringsValue("registry.example.test/platform/*")}
	pinned := m3BirthTree(t, capabilities)
	pinned["Dockerfile"] = []byte("FROM registry.example.test/platform/certificates:latest AS certs\n\n" +
		"FROM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS build\n" +
		"RUN go version\n")
	for _, tc := range []struct {
		name     string
		files    map[string][]byte
		options  model.Options
		findings int
	}{
		{"red every base by tag", m3BirthTree(t, capabilities), nil, 3},
		{"red the listed registry is left, the others found", m3BirthTree(t, capabilities), listed, 2},
		{"near listed registry and digests elsewhere", pinned, listed, 0},
	} {
		result := m3Shape(t, policyfiles.ImageDigest(), tc.files, capabilities, tc.options)
		if result.Status.Kind != model.StatusMeasured || len(result.Findings) != tc.findings {
			t.Fatalf("%s: status %+v, findings %+v", tc.name, result.Status, result.Findings)
		}
		for _, finding := range result.Findings {
			if tc.options != nil && finding.Line == 1 {
				t.Fatalf("%s: the listed registry was found: %+v", tc.name, finding)
			}
		}
	}
}
