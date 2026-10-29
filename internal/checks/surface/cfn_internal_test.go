package surface

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// httpsOnlyCount runs the check's predicate over a template and counts
// the buckets it reports.
func httpsOnlyCount(src string) int {
	f := source{path: "infra/a.yml", src: []byte(src), info: detect.FileInfo{Language: detect.CloudFormation}}
	return len(judgeHTTPSOnly(f, model.Input{}))
}

func TestHTTPSOnlyReadsTheShapesOfATemplate(t *testing.T) {
	t.Parallel()
	const deny = "      PolicyDocument:\n        Statement: &s\n          - Effect: Deny\n            Action: [\"s3:*\", 7]\n" +
		"            Condition: {Bool: {aws:SecureTransport: [\"FALSE\"]}}\n"
	tests := []struct {
		name string
		src  string
		want int
	}{
		{"an empty document", "", 0},
		{"a document that is a list", "- a\n", 0},
		{"resources that are a list", "Resources: [a]\n", 0},
		{"a resource that is a list", "Resources:\n  B: [x]\n", 0},
		{"an anchored bucket", "Base: &b\n  Type: AWS::S3::Bucket\nResources:\n  B: *b\n", 1},
		{"a type that is a mapping", "Resources:\n  B:\n    Type: {x: y}\n", 0},
		{"a literal bucket name", "Resources:\n  B:\n    Type: AWS::S3::Bucket\n  P:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n      Bucket: B\n" + deny, 1},
		{"no bucket property", "Resources:\n  B:\n    Type: AWS::S3::Bucket\n  P:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n" + deny, 1},
		{"statements as a scalar", "Resources:\n  B:\n    Type: AWS::S3::Bucket\n  P:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n      Bucket: !Ref B\n      PolicyDocument:\n        Statement: none\n", 1},
		{"no statement", "Resources:\n  B:\n    Type: AWS::S3::Bucket\n  P:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n      Bucket: !Ref B\n", 1},
		{"an action as a mapping", "Resources:\n  B:\n    Type: AWS::S3::Bucket\n  P:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n      Bucket: !Ref B\n      PolicyDocument:\n        Statement:\n          Effect: Deny\n          Action: {a: b}\n", 1},
		{"a list and an aliased statement", "Resources:\n  B:\n    Type: AWS::S3::Bucket\n  P:\n    Type: AWS::S3::BucketPolicy\n    Properties:\n      Bucket: !Ref B\n" + deny, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := httpsOnlyCount(tt.src); got != tt.want {
				t.Fatalf("buckets = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFromReferenceWithoutAReference(t *testing.T) {
	t.Parallel()
	if got := fromReference("--platform=linux/amd64"); got != "" {
		t.Fatalf("reference = %q, want none", got)
	}
}
