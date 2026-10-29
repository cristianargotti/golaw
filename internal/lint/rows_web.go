package lint

import _ "embed"

// webRulemap is the public rule map of the web surface (M6.4.3).
//
//go:embed rulemap/web.yml
var webRulemap []byte

// webRows answers the rows of the web rule map. TestWebRowsAreValid proves
// the embedded map decodes, so the empty answer on an error never ships.
func webRows() []Row {
	rows, err := DecodeRows(webRulemap)
	if err != nil {
		return nil
	}
	return rows
}
