package assets

import "go/version"

const toolsGoFloor = "1.26.6"

// ToolsGo covers every pinned tool without raising the adopter's root directive.
func (v Values) ToolsGo() string {
	if version.Compare("go"+v.GoVersion, "go"+toolsGoFloor) < 0 {
		return toolsGoFloor
	}
	return v.GoVersion
}
