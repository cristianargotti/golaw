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

func report(out io.Writer, rows []row, version, digest string) bool {
	if _, err := fmt.Fprintln(out,
		"CATEGORY\tAGENT\tOS\tVERSION\tBINARY\tSTATUS\tCASE\tDETAIL"); err != nil {
		return false
	}
	passed := true
	for _, item := range rows {
		if item.status == statusFail || item.status == statusSkip && item.required {
			passed = false
		}
		if item.status != statusPass && item.status != statusFail && item.status != statusSkip {
			passed = false
		}
		detail := strings.Join(strings.Fields(item.detail), " ")
		if _, err := fmt.Fprintf(out, "%s\tclaude\t%s\t%s\t%s\t%s\t%s\t%s\n",
			item.category, runtime.GOOS, version, digest,
			item.status, item.name, detail); err != nil {
			return false
		}
	}
	return passed
}
