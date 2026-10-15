package assets

import "errors"

// ErrManifest prevents an incomplete bundle from appearing installable.
var ErrManifest = errors.New("invalid asset manifest")
