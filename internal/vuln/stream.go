// Package vuln reads what govulncheck reports and judges it by the dated
// vulnerability waivers of golaw.yml (A6), for the pre-push rung and the
// scheduled vulnerability lane alike: an advisory counts when its trace
// reaches a function of the code, and a waiver holds through the whole of
// its last day in UTC.
package vuln

import (
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
// whose trace reaches a function, each once, sorted by id.
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

// read decodes the whole stream: the scanner version of its config message
// and the reachable advisories.
func read(r io.Reader) (string, []Finding, error) {
	version, configured := "", false
	byID := map[string]Finding{}
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
		// A later report of an advisory replaces an earlier one, so the
		// lane names the fix of the last report as it always did.
		if f := m.Finding; f != nil && len(f.Trace) > 0 && f.Trace[0].Function != "" {
			byID[f.OSV] = Finding{ID: f.OSV, Module: f.Trace[0].Module, Fixed: f.FixedVersion}
		}
	}
	if !configured {
		return "", nil, fmt.Errorf("%w: no config message", ErrStream)
	}
	found := make([]Finding, 0, len(byID))
	for _, f := range byID {
		found = append(found, f)
	}
	slices.SortFunc(found, func(a, b Finding) int { return strings.Compare(a.ID, b.ID) })
	return version, found, nil
}
