package main

import "errors"

// ErrWaiver marks a waiver entry the tool refuses: without an advisory id,
// without a record or with a day it cannot parse.
var ErrWaiver = errors.New("invalid vulnerability waiver")
