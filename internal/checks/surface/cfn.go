package surface

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

// The resource types the check reads.
const (
	typeBucket       = "AWS::S3::Bucket"
	typeBucketPolicy = "AWS::S3::BucketPolicy"
)

// CFNBucketHTTPSOnly reports an S3 bucket that no bucket policy of the
// same template holds to HTTPS (M6.7 check 9). A policy in another stack
// makes this a false finding, which is why the check never denies; the
// person answers it with an exceptions entry.
func CFNBucketHTTPSOnly() model.Check {
	return check{
		spec:      spec("cfn.bucket-https-only"),
		languages: []detect.Language{detect.CloudFormation},
		judge:     judgeHTTPSOnly,
	}
}

// bucketKey is one bucket of a template: its logical id and the line of
// the key that names it.
type bucketKey struct {
	id   string
	line int
}

// judgeHTTPSOnly answers one finding per bucket no policy protects, at
// the line of its logical id. Smallest behavior: a document that does not
// decode gives nothing, as the scaffolding treats an unparseable template.
func judgeHTTPSOnly(f source, _ model.Input) []model.Finding {
	resources := templateResources(f.src)
	if resources == nil {
		return nil
	}
	var buckets []bucketKey
	protected := map[string]bool{}
	for i := 0; i+1 < len(resources.Content); i += 2 {
		key, resource := resources.Content[i], resolve(resources.Content[i+1])
		switch scalarValue(mapValue(resource, "Type")) {
		case typeBucket:
			buckets = append(buckets, bucketKey{id: key.Value, line: key.Line})
		case typeBucketPolicy:
			if id, ok := protects(resource); ok {
				protected[id] = true
			}
		}
	}
	var out []model.Finding
	for _, b := range buckets {
		if protected[b.id] {
			continue
		}
		message := fmt.Sprintf("bucket %s has no bucket policy in this template that denies every action when aws:SecureTransport is false (Sonar S6249): add an AWS::S3::BucketPolicy whose Bucket is !Ref %s with that Deny statement.", b.id, b.id)
		out = append(out, f.finding("bucket-https-only", b.line, message, sonarKey(detect.CloudFormation, "S6249")))
	}
	return out
}

// templateResources answers the top-level Resources mapping of a
// template, nil when the document does not decode or holds none. The
// decoder reads JSON too and keeps the intrinsic short forms as tagged
// scalars.
func templateResources(src []byte) *yaml.Node {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil || doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil
	}
	resources := mapValue(resolve(doc.Content[0]), "Resources")
	if resources == nil || resources.Kind != yaml.MappingNode {
		return nil
	}
	return resources
}

// protects answers the bucket a policy holds to HTTPS: the one its Bucket
// refers to, when one of its statements denies every action over plain
// HTTP.
func protects(policy *yaml.Node) (string, bool) {
	properties := mapValue(policy, "Properties")
	id, ok := refers(mapValue(properties, "Bucket"))
	if !ok {
		return "", false
	}
	return id, slices.ContainsFunc(statements(mapValue(mapValue(properties, "PolicyDocument"), "Statement")), denies)
}
