package main

import "errors"

var (
	// ErrWaiver marks a waiver entry the tool refuses: without an advisory id,
	// without a record or with a day it cannot parse.
	ErrWaiver = errors.New("invalid vulnerability waiver")
	// ErrNoConfig marks a stream without the config message govulncheck
	// writes first: a scanner that fails before that line leaves none.
	ErrNoConfig = errors.New("govulncheck stream without its config message")
)
