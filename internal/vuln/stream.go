// Package vuln reads what govulncheck reports and judges it by the dated
// vulnerability waivers of golaw.yml (A6), for the pre-push rung and the
// scheduled vulnerability lane alike: an advisory counts when its trace
// reaches a function of the code, and a waiver holds through the whole of
// its last day in UTC.
package vuln

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Finding is one advisory govulncheck reports reachable from the code.
type Finding struct {
	ID     string // GO-2026-6603
	Module string // the module of the vulnerable symbol, stdlib for the standard library
	Fixed  string // the version that fixes it, empty when none exists
}

// message is one object of the stream; only the config and the findings
// matter here, every other kind decodes into nothing.
type message struct {
	Config  *config  `json:"config"`
	Finding *finding `json:"finding"`
}

type config struct {
	ScannerVersion string `json:"scanner_version"`
}

type finding struct {
	OSV          string  `json:"osv"`
	FixedVersion string  `json:"fixed_version"`
	Trace        []frame `json:"trace"`
}

// frame is one step of a trace; govulncheck orders a trace from the
// vulnerable symbol to the entry point, so the first frame names a
// function only when the code calls the symbol.
type frame struct {
	Module   string `json:"module"`
	Function string `json:"function"`
}

// Reachable reads the JSON stream of govulncheck and answers the advisories
// whose trace reaches a function, one finding per module and fix of an
// advisory, sorted by id, module and fix: an advisory reachable in two
// modules needs both fixes, and govulncheck reports in no fixed order.
func Reachable(r io.Reader) ([]Finding, error) {
	_, found, err := read(r)
	return found, err
}

// Scanner answers the version of govulncheck that wrote the stream, as its
// config message names it.
func Scanner(r io.Reader) (string, error) {
	version, _, err := read(r)
	return version, err
}

// read decodes the stream to its end, because a stream cut short or one
// without the config message govulncheck writes first comes from no
// complete scan and must never read as clean.
func read(r io.Reader) (string, []Finding, error) {
	version, configured := "", false
	seen := map[Finding]bool{}
	decoder := json.NewDecoder(r)
	for {
		var m message
		err := decoder.Decode(&m)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", nil, fmt.Errorf("%w: %w", ErrStream, err)
		}
		if m.Config != nil {
			version, configured = m.Config.ScannerVersion, true
		}
		if f := m.Finding; f != nil && len(f.Trace) > 0 && f.Trace[0].Function != "" {
			seen[Finding{ID: f.OSV, Module: f.Trace[0].Module, Fixed: f.FixedVersion}] = true
		}
	}
	if !configured {
		return "", nil, fmt.Errorf("%w: no config message", ErrStream)
	}
	found := make([]Finding, 0, len(seen))
	for f := range seen {
		found = append(found, f)
	}
	slices.SortFunc(found, func(a, b Finding) int {
		return cmp.Or(strings.Compare(a.ID, b.ID), strings.Compare(a.Module, b.Module), strings.Compare(a.Fixed, b.Fixed))
	})
	return version, found, nil
}
