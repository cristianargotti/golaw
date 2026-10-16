package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"
)

// module is one object of the stream go list -m -json all prints.
type module struct {
	Path    string     `json:"Path"`
	Version string     `json:"Version"`
	Time    *time.Time `json:"Time"`
	Main    bool       `json:"Main"`
	Replace *module    `json:"Replace"`
}

// decode reads the stream object by object, because go list -m -json
// prints one JSON object per module and no enclosing array.
func decode(stream []byte) ([]module, error) {
	decoder := json.NewDecoder(bytes.NewReader(stream))
	var modules []module
	for {
		var m module
		err := decoder.Decode(&m)
		if errors.Is(err, io.EOF) {
			return modules, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrStream, err)
		}
		modules = append(modules, m)
	}
}

// judge names every module of the graph that entered too early: one
// published less than minimum before now, one without a publication time
// (an unknown age is not old enough) and an allowed entry the graph does
// not hold, so the exception list only shrinks. The main module is the
// repository itself and is never judged; the count is of the judged ones.
func judge(modules []module, now time.Time, minimum time.Duration, allowed []string) ([]string, int) {
	var lines []string
	judged := 0
	used := make(map[string]bool, len(allowed))
	for _, m := range modules {
		if m.Main {
			continue
		}
		judged++
		if m.Replace != nil {
			m = *m.Replace
		}
		id := m.Path + "@" + m.Version
		if slices.Contains(allowed, id) {
			used[id] = true
			continue
		}
		switch {
		case m.Version == "" || m.Time == nil:
			lines = append(lines, id+" has no publication time")
		case now.Sub(*m.Time) < minimum:
			lines = append(lines, fmt.Sprintf("%s was published %s, less than %s ago",
				id, m.Time.UTC().Format(time.DateOnly), days(minimum)))
		}
	}
	for _, id := range allowed {
		if !used[id] {
			lines = append(lines, "-allow "+id+" names no module of the graph")
		}
	}
	return lines, judged
}

// days renders a minimum age the way the flag takes it.
func days(minimum time.Duration) string {
	return fmt.Sprintf("%d days", int(minimum/day))
}
