package app

import "errors"

// ErrNotFound is declared next to its call site.
var ErrNotFound = errors.New("not found")

// Find answers the sentinel.
func Find() error {
	return ErrNotFound
}
