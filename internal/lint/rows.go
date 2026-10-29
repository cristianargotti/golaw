package lint

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

// Row is one line of the rule map: a Sonar way key and the tool rule that
// stands in for it (M6.4.3).
type Row struct {
	// Sonar is the Sonar way key the rule answers to: javascript:S2681.
	Sonar string `yaml:"sonar"`
	// Tool is the pinned tool that reports the rule.
	Tool string `yaml:"tool"`
	// Rule is the rule as the tool names it.
	Rule string `yaml:"rule"`
	// Where says where the rule runs: H the hook, N a native check, L the
	// lint target.
	Where []string `yaml:"where"`
	// Match is exact when the tool rule reports what the Sonar rule
	// reports, close when it reports the same class with other edges.
	Match string `yaml:"match"`
	// Set is the lowest set that enables the rule: parity or strict.
	Set string `yaml:"set"`
}

// The shapes of a row's Sonar key and tool name.
var (
	sonarKey = regexp.MustCompile(`^[A-Za-z]+:S\d+$`)
	rowTool  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// rowPlaces, rowMatches and rowSets are the words a row may name; functions
// keep them out of reach of a caller that would change them.
func rowPlaces() []string  { return []string{"H", "N", "L"} }
func rowMatches() []string { return []string{"exact", "close"} }
func rowSets() []string    { return []string{model.SurfaceParity, model.SurfaceStrict} }

// DecodeRows reads a rule map, a YAML sequence of rows, strictly: an
// unknown key, a mapping at the top, an invalid field or a tool rule mapped
// twice is ErrRows, because a row golaw cannot trust would report a Sonar
// key the pipeline never raises.
func DecodeRows(data []byte) ([]Row, error) {
	var rows []Row
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&rows); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRows, err)
	}
	seen := make(map[[2]string]bool, len(rows))
	for i, row := range rows {
		if err := validRow(i, row, seen); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// validRow checks one row and records its tool rule; one tool rule maps to
// one Sonar key, while one Sonar key may have several rows.
func validRow(index int, row Row, seen map[[2]string]bool) error {
	n := index + 1
	switch {
	case !sonarKey.MatchString(row.Sonar):
		return fmt.Errorf("%w: row %d: sonar key %q is not language:S<number>", ErrRows, n, row.Sonar)
	case !rowTool.MatchString(row.Tool):
		return fmt.Errorf("%w: row %d: tool %q", ErrRows, n, row.Tool)
	case row.Rule == "":
		return fmt.Errorf("%w: row %d: %s without a rule", ErrRows, n, row.Sonar)
	case len(row.Where) == 0:
		return fmt.Errorf("%w: row %d: %s/%s runs nowhere", ErrRows, n, row.Tool, row.Rule)
	case !slices.Contains(rowMatches(), row.Match):
		return fmt.Errorf("%w: row %d: match %q is not exact or close", ErrRows, n, row.Match)
	case !slices.Contains(rowSets(), row.Set):
		return fmt.Errorf("%w: row %d: set %q is not parity or strict", ErrRows, n, row.Set)
	}
	places := make(map[string]bool, len(row.Where))
	for _, place := range row.Where {
		if !slices.Contains(rowPlaces(), place) || places[place] {
			return fmt.Errorf("%w: row %d: place %q is unknown or repeated", ErrRows, n, place)
		}
		places[place] = true
	}
	key := [2]string{row.Tool, row.Rule}
	if seen[key] {
		return fmt.Errorf("%w: row %d: %s/%s is mapped twice", ErrRows, n, row.Tool, row.Rule)
	}
	seen[key] = true
	return nil
}
