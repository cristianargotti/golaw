package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"time"
)

// message is one object of the govulncheck JSON stream; only the config and
// the findings matter here, every other kind is decoded into nothing.
type message struct {
	Config  *struct{} `json:"config"`
	Finding *finding  `json:"finding"`
}

type finding struct {
	OSV          string  `json:"osv"`
	FixedVersion string  `json:"fixed_version"`
	Trace        []frame `json:"trace"`
}

// frame is one step of a trace; a function means the code reaches the
// vulnerable symbol, a module or package alone means it only depends on it.
type frame struct {
	Module   string `json:"module"`
	Package  string `json:"package"`
	Function string `json:"function"`
}

// readReachable requires the config message because govulncheck writes it
// before anything else: a stream without it comes from a scan that never
// started, and reading no finding there as clean would pass a failed scan.
func readReachable(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	configured := false
	decoder := json.NewDecoder(r)
	for {
		var m message
		err := decoder.Decode(&m)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decode govulncheck stream: %w", err)
		}
		configured = configured || m.Config != nil
		if m.Finding != nil && m.Finding.reachable() {
			out[m.Finding.OSV] = m.Finding.FixedVersion
		}
	}
	if !configured {
		return nil, ErrNoConfig
	}
	return out, nil
}

// reachable looks at the first frame alone because govulncheck orders a
// trace from the vulnerable symbol to the entry point, so that frame names
// a function only when the code calls the symbol.
func (f *finding) reachable() bool {
	return len(f.Trace) > 0 && f.Trace[0].Function != ""
}

// judge renders one line per reachable advisory and per expired waiver and
// reports whether anything is red: a reachable advisory without an
// unexpired waiver, or a waiver past its day.
func judge(reachable map[string]string, waivers map[string]entry, today time.Time) ([]string, bool) {
	var lines []string
	red := false
	for _, id := range slices.Sorted(maps.Keys(reachable)) {
		w, ok := waivers[id]
		switch {
		case ok && !today.After(w.until):
			lines = append(lines, id+" waived until "+w.until.Format(time.DateOnly)+" ("+w.record+")")
		default:
			lines = append(lines, id+" reachable, fixed in "+fixedOr(reachable[id]))
			red = true
		}
	}
	for _, id := range slices.Sorted(maps.Keys(waivers)) {
		if today.After(waivers[id].until) {
			lines = append(lines, "waiver "+id+" expired on "+waivers[id].until.Format(time.DateOnly)+", remove it")
			red = true
		}
	}
	if len(reachable) == 0 {
		lines = append([]string{"no reachable vulnerability"}, lines...)
	}
	return lines, red
}

// fixedOr still renders an advisory without a fix, because that is the
// case a dated waiver exists for.
func fixedOr(version string) string {
	if version == "" {
		return "no version yet"
	}
	return version
}
