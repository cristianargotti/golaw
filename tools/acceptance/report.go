package main

import "io"

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
	return reportEvidence(out, rows, version, digest, nil)
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
