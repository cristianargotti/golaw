package main

import "errors"

// The sentinels of the command: a run without the package to read, a
// README whose tables it cannot find, and a directory that holds another
// package.
var (
	errUsage   = errors.New("-package names the installed eslint-plugin-sonarjs directory")
	errReadme  = errors.New("README layout not recognized")
	errPackage = errors.New("not the eslint-plugin-sonarjs package")
)
