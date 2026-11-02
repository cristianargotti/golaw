package genheader

import "bytes"

// SameIgnoringVersion reports whether two renderings of a file are the same
// but for the kit version of their headers (M7.4.1): a kit upgrade alone must
// drift nothing, so the version token is rewritten only with the file, while
// another kind, another template or any other byte still differs.
func SameIgnoringVersion(a, b []byte) bool {
	ha, okA := Parse(a)
	hb, okB := Parse(b)
	if !okA || !okB {
		return bytes.Equal(a, b)
	}
	if ha.Kind != hb.Kind || ha.Template != hb.Template {
		return false
	}
	_, restA, _ := bytes.Cut(a, []byte("\n"))
	_, restB, _ := bytes.Cut(b, []byte("\n"))
	return bytes.Equal(restA, restB)
}
