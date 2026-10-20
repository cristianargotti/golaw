package assets

import "errors"

// ErrManifest prevents an incomplete bundle from appearing installable.
var ErrManifest = errors.New("invalid asset manifest")

// ErrRender prevents partial or invalid output from being applied.
var ErrRender = errors.New("template render failed")
