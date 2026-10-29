package surface_test

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
)

// template is a CloudFormation template written with the short forms the
// detector must accept as scalars.
const template = `AWSTemplateFormatVersion: "2010-09-09"
Resources:
  ArtifactBucket:
    Type: AWS::S3::Bucket
Outputs:
  Name:
    Value: !Ref ArtifactBucket
  Arn:
    Value: !GetAtt ArtifactBucket.Arn
  Url:
    Value: !Sub "https://${ArtifactBucket}.example.test"
`

func TestDetectSettlesTheLanguageFromTheNameOrTheBytes(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		path    string
		content string
		want    surface.Language
	}{
		"a node shebang":                            {path: "scripts/run", content: "#!/usr/bin/env node\nconsole.log(1)\n", want: surface.JavaScript},
		"a node shebang through env -S":             {path: "bin/tool", content: "#!/usr/bin/env -S node --no-warnings\n", want: surface.JavaScript},
		"a node shebang after an assignment":        {path: "bin/env-tool", content: "#!/usr/bin/env NODE_ENV=test node\n", want: surface.JavaScript},
		"a versioned python shebang":                {path: "bin/check", content: "#!/usr/bin/python3.12\nprint(1)\n", want: surface.Python},
		"a python shebang through env":              {path: "bin/py", content: "#!/usr/bin/env python\n", want: surface.Python},
		"a bash shebang":                            {path: "bin/deploy", content: "#!/bin/bash\nset -eu\n", want: surface.Shell},
		"a sh shebang with CRLF":                    {path: "bin/up", content: "#!/bin/sh\r\nset -eu\r\n", want: surface.Shell},
		"a dash shebang":                            {path: "bin/down", content: "#!/bin/dash\n", want: surface.Shell},
		"a zsh shebang":                             {path: "bin/z", content: "#!/usr/bin/env zsh\n", want: surface.Shell},
		"a perl shebang (near)":                     {path: "bin/perl", content: "#!/usr/bin/perl\n", want: ""},
		"a pythonic interpreter (near)":             {path: "bin/pythonic", content: "#!/usr/bin/pythonic\n", want: ""},
		"a bare env (near)":                         {path: "bin/env-only", content: "#!/usr/bin/env\n", want: ""},
		"an empty shebang (near)":                   {path: "bin/empty", content: "#!\n", want: ""},
		"a comment that names node (near)":          {path: "bin/notes", content: "# runs under node\n", want: ""},
		"a license without an extension (near)":     {path: "LICENSE", content: "MIT License\n", want: ""},
		"a template with short forms":               {path: "infra/10-base.yml", content: template, want: surface.CloudFormation},
		"a Resources map without a version":         {path: "infra/queue.yaml", content: "Resources:\n  Q:\n    Type: AWS::SQS::Queue\n", want: surface.CloudFormation},
		"a JSON template":                           {path: "infra/stack.json", content: `{"AWSTemplateFormatVersion":"2010-09-09","Resources":{}}`, want: surface.CloudFormation},
		"a .template file":                          {path: "infra/stack.template", content: `{"Resources":{"B":{"Type":"AWS::S3::Bucket"}}}`, want: surface.CloudFormation},
		"a compose file (near)":                     {path: "deploy/compose.yaml", content: "services:\n  app:\n    image: app\n", want: surface.YAML},
		"a document that only quotes a type (near)": {path: "docs/aws.yaml", content: "notes: \"AWS::S3::Bucket is a type\"\n", want: surface.YAML},
		"one resource typed AWS:: makes a template": {path: "deploy/quota.yaml", content: "Resources:\n  cpu: 2\n  pool:\n    Type: AWS::none\n", want: surface.CloudFormation},
		"resources typed by another cloud (near)":   {path: "deploy/res.yaml", content: "Resources:\n  pool:\n    Type: GCP::Pool\n  AWS::x: 1\n", want: surface.YAML},
		"a Helm template without markers (near)":    {path: "deploy/chart/templates/pod.yaml", content: "{{ if .Values.on }}\nkind: Pod\n{{ end }}\n", want: surface.YAML},
		"a template in the second document (near)":  {path: "deploy/two.yaml", content: "kind: A\n---\nAWSTemplateFormatVersion: x\n", want: surface.YAML},
		"a list at the top (near)":                  {path: "deploy/list.yaml", content: "- AWS::S3::Bucket\n", want: surface.YAML},
		"a comment that names a type (near)":        {path: "deploy/note.yaml", content: "# AWS::S3::Bucket\n", want: surface.YAML},
		"a Resources list (near)":                   {path: "deploy/list2.yaml", content: "Resources:\n  - AWS::S3::Bucket\n", want: surface.YAML},
		"an empty yaml file (near)":                 {path: "deploy/empty.yml", want: surface.YAML},
		"parameters of a stack (near)":              {path: "infra/params.json", content: `[{"ParameterKey":"Env","ParameterValue":"stg"}]`, want: ""},
		"a lock file (near)":                        {path: "package-lock.json", content: `{"name":"web","lockfileVersion":3}`, want: ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := surface.Detect(tc.path, []byte(tc.content))
			if err != nil || got.Language != tc.want || got.Path != tc.path {
				t.Fatalf("Detect(%q) = %+v, %v, want %q", tc.path, got, err, tc.want)
			}
		})
	}
}

func TestDetectClassifiesAndReadsTheRegionsOfHTML(t *testing.T) {
	t.Parallel()
	got, err := surface.Detect("views/panel.html", []byte("<style>\nb {}\n</style>\n<script>\nrun();\n</script>\n"))
	if err != nil || got.Language != surface.HTML || got.Class != surface.ClassSource || len(got.Regions) != 2 {
		t.Fatalf("Detect = %+v, %v, want html source with two regions", got, err)
	}
	minified, err := surface.Detect("views/charts.min.js", []byte("a()"))
	if err != nil || minified.Class != surface.ClassMinified || minified.Regions != nil {
		t.Fatalf("Detect = %+v, %v, want a minified script without regions", minified, err)
	}
	plain, err := surface.Detect("cmd/hub/main.go", []byte("package main\n"))
	if err != nil || plain.Language != "" || plain.Class != 0 {
		t.Fatalf("Detect of Go = %+v, %v, want no surface language", plain, err)
	}
}

func TestDetectNamesATemplateItCannotParse(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		path    string
		content string
		want    surface.Language
	}{
		"a broken yaml template keeps the yaml language": {path: "infra/broken.yml", content: "AWSTemplateFormatVersion: [\n", want: surface.YAML},
		"a broken JSON template has no language":         {path: "infra/broken.json", content: `{"Resources": {"B": {"Type": "AWS::S3::Bucket"`, want: ""},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := surface.Detect(tc.path, []byte(tc.content))
			if !errors.Is(err, surface.ErrCloudFormation) || got.Language != tc.want {
				t.Fatalf("Detect(%q) = %+v, %v, want %q and ErrCloudFormation", tc.path, got, err, tc.want)
			}
		})
	}
}
