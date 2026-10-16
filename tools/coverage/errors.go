package main

import "errors"

// ErrModule marks a go.mod without a module line.
var ErrModule = errors.New("go.mod has no module line")

// ErrProfile marks a cover profile line the parser cannot read.
var ErrProfile = errors.New("invalid cover profile")
