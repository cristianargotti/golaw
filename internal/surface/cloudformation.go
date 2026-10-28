package surface

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// The markers without which no CloudFormation template exists: a file
// that holds neither is never parsed, so a Helm template or a lock file
// costs a byte search and is never Unmeasured.
const (
	versionKey     = "AWSTemplateFormatVersion"
	resourcePrefix = "AWS::"
)

// template reports whether YAML or JSON bytes are a CloudFormation
// template (M6.3.1): the first document's top level holds the version key,
// or a Resources map with one Type starting AWS::. Decoding into a node
// accepts the intrinsic short forms (!Ref, !Sub, !GetAtt) as tagged
// scalars, so no custom tag needs a resolver.
func template(content []byte) (bool, error) {
	if !bytes.Contains(content, []byte(versionKey)) && !bytes.Contains(content, []byte(resourcePrefix)) {
		return false, nil
	}
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(content)).Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		return false, fmt.Errorf("%w: %s", ErrCloudFormation, strings.Join(strings.Fields(err.Error()), " "))
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false, nil
	}
	top := doc.Content[0]
	for i := 0; i+1 < len(top.Content); i += 2 {
		switch top.Content[i].Value {
		case versionKey:
			return true, nil
		case "Resources":
			if awsResources(top.Content[i+1]) {
				return true, nil
			}
		}
	}
	return false, nil
}

// awsResources reports a Resources map with one resource whose Type starts
// AWS::.
func awsResources(resources *yaml.Node) bool {
	if resources.Kind != yaml.MappingNode {
		return false
	}
	for i := 1; i < len(resources.Content); i += 2 {
		resource := resources.Content[i]
		if resource.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(resource.Content); j += 2 {
			if resource.Content[j].Value == "Type" && strings.HasPrefix(resource.Content[j+1].Value, resourcePrefix) {
				return true
			}
		}
	}
	return false
}
