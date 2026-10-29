package lint

import (
	_ "embed" // the rule map travels inside the binary
	"fmt"
)

// infraRuleMap is the rule map of the infrastructure surface.
//
//go:embed rulemap/infra.yml
var infraRuleMap []byte

// infraRows answers the rows of the map. The file is embedded and its test
// decodes it, so a failure here is a build defect, never a runtime state.
func infraRows() []Row {
	rows, err := DecodeRows(infraRuleMap)
	if err != nil {
		panic(fmt.Errorf("rulemap/infra.yml: %w", err))
	}
	return rows
}
