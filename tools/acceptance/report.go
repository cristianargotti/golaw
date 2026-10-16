package main

import (
	"fmt"
	"io"
	"runtime"
	"strings"
)

const (
	statusPass = "PASS"
	statusFail = "FAIL"
	statusSkip = "SKIP"
)

type row struct {
	category string
	name     string
	status   string
	detail   string
	required bool
}

func measured(category, name string, err error) row {
	got := row{category, name, statusPass, "verified offline", true}
	if err != nil {
		got.status, got.detail = statusFail, err.Error()
	}
	return got
}

// report prints one line per row, with whether the milestone requires it
// (section 10.3 item 6), and closes a failed run with the rows that fail
// it, so a release run stopped by a skipped required row names that row.
func report(out io.Writer, rows []row, version, digest string) bool {
	if _, err := fmt.Fprintln(out,
		"CATEGORY\tAGENT\tOS\tVERSION\tBINARY\tSTATUS\tREQUIRED\tCASE\tDETAIL"); err != nil {
		return false
	}
	var failing []string
	for _, item := range rows {
		if blocks(item) {
			failing = append(failing, failure(item))
		}
		need := "optional"
		if item.required {
			need = "required"
		}
		detail := strings.Join(strings.Fields(item.detail), " ")
		if _, err := fmt.Fprintf(out, "%s\tclaude\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			item.category, runtime.GOOS, version, digest,
			item.status, need, item.name, detail); err != nil {
			return false
		}
	}
	if len(failing) == 0 {
		return true
	}
	_, _ = fmt.Fprintf(out, "rows that fail the run: %s\n", strings.Join(failing, ", ")) //nolint:errcheck // the run fails whatever the write answers
	return false
}

// blocks reports whether a row fails the run: a FAIL, a SKIP of a
// required row, or a status that is none of the three.
func blocks(item row) bool {
	switch item.status {
	case statusPass:
		return false
	case statusSkip:
		return item.required
	default:
		return true
	}
}

// failure names a row that fails the run; a skipped one says it was
// required, the reason it counts.
func failure(item row) string {
	name := item.category + "/" + item.name + " " + item.status
	if item.status == statusSkip {
		name += " required"
	}
	return name
}
