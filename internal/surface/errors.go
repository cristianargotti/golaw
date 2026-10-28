package surface

import "errors"

// ErrCloudFormation marks a YAML or JSON file that carries the
// CloudFormation markers and does not parse: whether it is a template is
// unknown, so the caller answers Unmeasured naming the path.
var ErrCloudFormation = errors.New("cloudformation template unreadable")
