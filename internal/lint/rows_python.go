package lint

import _ "embed"

// pythonRulemap is the rule map of the Python tools, one row per Sonar way
// key and local rule.
//
//go:embed rulemap/python.yml
var pythonRulemap []byte

// pythonRows answers the Python rows; the test proves the embedded file
// decodes, so a decode error answers no row rather than a partial map.
func pythonRows() []Row {
	rows, err := DecodeRows(pythonRulemap)
	if err != nil {
		return nil
	}
	return rows
}
