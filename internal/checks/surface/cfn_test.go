package surface_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/surface"
	"github.com/cristianargotti/golaw/internal/model"
)

const bucket = "AWSTemplateFormatVersion: \"2010-09-09\"\nResources:\n  ArtifactBucket:\n    Type: AWS::S3::Bucket\n"

// bucketPolicy is a bucket policy of ArtifactBucket with one statement.
func bucketPolicy(statement string) string {
	return "  ArtifactBucketPolicy:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n      Bucket: !Ref ArtifactBucket\n" +
		"      PolicyDocument:\n        Statement:\n" + statement
}

const httpsOnly = "          - Effect: Deny\n            Principal: \"*\"\n            Action: \"s3:*\"\n" +
	"            Resource: !Sub \"${ArtifactBucket.Arn}/*\"\n            Condition:\n              Bool:\n                aws:SecureTransport: \"false\"\n"

func TestCFNBucketHTTPSOnlyFindsAnOpenBucket(t *testing.T) {
	t.Parallel()
	runCases(t, surface.CFNBucketHTTPSOnly(), []fileCase{
		{"a bucket with no policy", "infra/10-base.yml", bucket, []string{"infra/10-base.yml:3 bucket-https-only"}},
		{"a json template", "infra/logs.json", "{\"AWSTemplateFormatVersion\": \"2010-09-09\", \"Resources\": {\"Logs\": {\"Type\": \"AWS::S3::Bucket\"}}}\n", []string{"infra/logs.json:1 bucket-https-only"}},
		{"a policy of another bucket", "infra/10-base.yml", bucket + "  LogBucket:\n    Type: AWS::S3::Bucket\n" + "  LogBucketPolicy:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n      Bucket: !Ref LogBucket\n      PolicyDocument:\n        Statement:\n" + httpsOnly, []string{"infra/10-base.yml:3 bucket-https-only"}},
		{"a policy that allows", "infra/10-base.yml", bucket + bucketPolicy("          - Effect: Allow\n            Action: \"s3:*\"\n            Condition:\n              Bool:\n                aws:SecureTransport: \"false\"\n"), []string{"infra/10-base.yml:3 bucket-https-only"}},
		{"a deny on secure transport", "infra/10-base.yml", bucket + bucketPolicy("          - Effect: Deny\n            Action: \"s3:*\"\n            Condition:\n              Bool:\n                aws:SecureTransport: \"true\"\n"), []string{"infra/10-base.yml:3 bucket-https-only"}},
		{"a deny of one action", "infra/10-base.yml", bucket + bucketPolicy("          - Effect: Deny\n            Action: s3:GetObject\n            Condition:\n              Bool:\n                aws:SecureTransport: \"false\"\n"), []string{"infra/10-base.yml:3 bucket-https-only"}},
	})
	runCases(t, surface.CFNBucketHTTPSOnly(), []fileCase{
		{"the https only policy", "infra/10-base.yml", bucket + bucketPolicy(httpsOnly), nil},
		{"an unquoted false", "infra/10-base.yml", bucket + bucketPolicy("          - Effect: Deny\n            Action: \"*\"\n            Condition:\n              Bool:\n                aws:SecureTransport: false\n"), nil},
		{"an action list", "infra/10-base.yml", bucket + bucketPolicy("          - Effect: Deny\n            Action: [\"s3:*\"]\n            Condition:\n              Bool:\n                aws:SecureTransport: [\"false\"]\n"), nil},
		{"one statement as a map", "infra/10-base.yml", bucket + "  P:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n      Bucket: {Ref: ArtifactBucket}\n      PolicyDocument:\n        Statement:\n          Effect: Deny\n          Action: \"s3:*\"\n          Condition: {Bool: {aws:SecureTransport: \"false\"}}\n", nil},
		{"a yaml that is no template", "deploy/buckets.yaml", "buckets:\n  - Type: AWS::S3::Bucket\n", nil},
		{"a document that does not parse", "infra/broken.yml", "Resources: [\n", nil},
	})
}

func TestCFNBucketHTTPSOnlyIsFeedback(t *testing.T) {
	t.Parallel()
	c := surface.CFNBucketHTTPSOnly()
	files := map[string]string{"infra/10-base.yml": bucket}
	want := []model.Finding{{
		Path: "infra/10-base.yml", Line: 3, Code: "bucket-https-only", Severity: model.SeverityFail, Sonar: "cloudformation:S6249",
		Message: "bucket ArtifactBucket has no bucket policy in this template that denies every action when aws:SecureTransport is false (Sonar S6249): add an AWS::S3::BucketPolicy whose Bucket is !Ref ArtifactBucket with that Deny statement.",
	}}
	if res := scanFiles(t, c, files); !slices.EqualFunc(res.Findings, want, sameFinding) {
		t.Fatalf("findings = %+v, want %+v", res.Findings, want)
	}
	if res := run(t, c, postInput(c, files)); len(res.Findings) != 1 || res.Findings[0].Severity != model.SeverityNote {
		t.Fatalf("post = %+v, want one note", res.Findings)
	}
}
