package genmap

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// matchLayout places dir in a layout pattern: both have the same segments
// and every segment but the one written {...} is equal; the answer is the
// segment the placeholder captures.
func matchLayout(pattern, dir string) (string, bool) {
	want, got := strings.Split(pattern, "/"), strings.Split(dir, "/")
	if pattern == "" || len(want) != len(got) {
		return "", false
	}
	captured, found := "", false
	for i, seg := range want {
		switch {
		case placeholder(seg):
			captured, found = got[i], got[i] != ""
		case seg != got[i]:
			return "", false
		}
	}
	return captured, found
}

// layoutDir is the directory a pattern names for one value.
func layoutDir(pattern, value string) string {
	segs := strings.Split(pattern, "/")
	for i, seg := range segs {
		if placeholder(seg) {
			segs[i] = value
		}
	}
	return strings.Join(segs, "/")
}

// placeholder reports whether a segment is a {name} of the layout.
func placeholder(seg string) bool {
	return len(seg) > 2 && strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}")
}

// layers lists the layout as the map's first line reads it, in a fixed
// order; a path the policy leaves empty is left out.
func layers(p model.GeneratePolicy) []string {
	pairs := [][2]string{
		{wordPort, p.Layout.Port},
		{"usecase", p.Layout.Usecase},
		{"adapter-sql", p.Layout.AdapterSQL},
		{"adapter-http", p.Layout.AdapterHTTP},
		{"handler", p.Layout.Handler},
		{"wiring", p.Layout.Wiring},
		{"support", p.Support},
		{"sqlc_out", p.ThirdParty.SQLCOut},
		{"openapi", p.ThirdParty.OpenAPI},
		{"openapi_out", p.ThirdParty.OpenAPIOut},
	}
	out := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		if pair[1] != "" {
			out = append(out, pair[0]+" "+pair[1])
		}
	}
	return out
}
