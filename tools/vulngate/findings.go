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

// message is one object of the govulncheck JSON stream; only findings
// matter here, every other kind is decoded into nothing.
type message struct {
	Finding *finding `json:"finding"`
}

// finding is one advisory hit with the call trace that reaches it.
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

// readReachable decodes the stream and keeps, per advisory, the fixed
// version of the findings whose trace reaches a function.
func readReachable(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	decoder := json.NewDecoder(r)
	for {
		var m message
		err := decoder.Decode(&m)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode govulncheck stream: %w", err)
		}
		if m.Finding != nil && m.Finding.reachable() {
			out[m.Finding.OSV] = m.Finding.FixedVersion
		}
	}
}

// reachable reports whether the trace reaches a function.
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

// fixedOr names the fixed version or says there is none yet.
func fixedOr(version string) string {
	if version == "" {
		return "no version yet"
	}
	return version
}
