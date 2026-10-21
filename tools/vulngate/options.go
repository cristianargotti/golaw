package main

import (
	"flag"
	"fmt"
	"io"
	"time"
)

// options are the flags of both modes, parsed apart from execute so each
// function stays within the size the repository allows.
type options struct {
	waiversPath string
	day         time.Time
	scan        bool
}

// parseOptions keeps -today required in stdin mode, where the lane passes
// the date, and defaults it to the UTC day of now under -scan.
func parseOptions(args []string, stderr io.Writer, now time.Time) (options, error) {
	flags := flag.NewFlagSet("vulngate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var opts options
	flags.StringVar(&opts.waiversPath, "waivers", "golaw.yml", "policy file holding vulnerability_waivers")
	flags.BoolVar(&opts.scan, "scan", false, "run the pinned govulncheck")
	today := flags.String("today", "", "date as YYYY-MM-DD, required without -scan")
	if err := flags.Parse(args); err != nil {
		return options{}, fmt.Errorf("parse options: %w", err)
	}
	if opts.scan && *today == "" {
		*today = now.UTC().Format(time.DateOnly)
	}
	day, err := time.Parse(time.DateOnly, *today)
	if err != nil {
		return options{}, fmt.Errorf("-today: %w", err)
	}
	opts.day = day
	return opts, nil
}
