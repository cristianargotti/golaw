package assets

import "errors"

// ErrManifest prevents an incomplete bundle from appearing installable.
var ErrManifest = errors.New("invalid asset manifest")

// ErrRender prevents partial or invalid output from being applied.
var ErrRender = errors.New("template render failed")

// ErrRunSteps prevents unsupported input from producing a partial replay.
var ErrRunSteps = errors.New("invalid CI run steps")

// The renderer's own refusals leave ErrRender to RenderContext, which adds it
// once with the source and the destination the reason names.
var (
	errDestination   = errors.New("invalid expanded destination")
	errDelimiter     = errors.New("unresolved template delimiter")
	errRequiredInput = errors.New("required input")
	errRequiredTool  = errors.New("required tool key")
)
