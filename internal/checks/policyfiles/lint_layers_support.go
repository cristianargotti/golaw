package policyfiles

import "github.com/cristianargotti/golaw/internal/model"

// Imports are module relative, so a support package outside internal/
// that generate.support names stays as reachable as the layers.
type layer struct {
	dir     string
	imports []string
}

// Every generated use case calls the support package (M7.4.3), so the
// application rule the asset renders allows it and only the domain stays
// closed to it.
func innerLayers(support string) []layer {
	if support == "" {
		support = model.DefaultGenerate().Support
	}
	return []layer{
		{dir: layerDomain, imports: []string{layerPrefix + layerDomain}},
		{dir: layerApplication, imports: []string{layerPrefix + layerDomain, layerPrefix + layerApplication, support}},
	}
}
