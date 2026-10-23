package profile

import (
	"errors"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

// readCause names a file the bundle could not be read from.
const readCause = "read %s: %v"

var (
	// ErrBundle marks a bundle whose layout, schema or a field value is
	// refused; every return wraps it under model.ErrPolicy (D8).
	ErrBundle = errors.New("invalid profile bundle")
	// ErrDigest marks a bundle whose recomputed digest differs from the
	// stamp or from the pin; every return wraps it under model.ErrPolicy.
	ErrDigest = errors.New("profile digest mismatch")
	// errRules prefixes every cause found inside the data file, so the
	// exported functions add the bundle and the profile name around it.
	errRules = errors.New(RulesFile)
)

// bundlef is ErrBundle under ErrPolicy with the cause formatted.
func bundlef(format string, args ...any) error {
	return fmt.Errorf("%w: %w: %s", model.ErrPolicy, ErrBundle, fmt.Sprintf(format, args...))
}

// digestf is ErrDigest under ErrPolicy with the cause formatted.
func digestf(format string, args ...any) error {
	return fmt.Errorf("%w: %w: %s", model.ErrPolicy, ErrDigest, fmt.Sprintf(format, args...))
}

// causef is a cause inside rules.yml, named by the file and the field.
func causef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errRules, fmt.Sprintf(format, args...))
}
