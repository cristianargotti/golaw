package surface

import "errors"

var (
	// ErrCloudFormation marks a YAML or JSON file that carries the
	// CloudFormation markers and does not parse: whether it is a template
	// is unknown, so the caller answers Unmeasured naming the path.
	ErrCloudFormation = errors.New("cloudformation template unreadable")
	// ErrPattern marks an analyzer pattern outside the Sonar glob grammar
	// golaw reads; the scope records it as Unmeasured, never as a match.
	ErrPattern = errors.New("analyzer pattern golaw cannot parse")
)
