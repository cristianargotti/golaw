package main

import "errors"

// ErrAcceptance identifies an unsuccessful acceptance assertion.
var ErrAcceptance = errors.New("acceptance assertion failed")
