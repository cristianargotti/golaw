package assets

import "go/version"

// toolsGoFloor is the highest go line of the go.mod files of the pins in
// runner.Toolchain (lefthook 2.1.12); a pin bump re-reads them and raises it.
const toolsGoFloor = "1.26.6"

// ToolsGo covers every pinned tool without raising the adopter's root directive.
func (v Values) ToolsGo() string {
	if version.Compare("go"+v.GoVersion, "go"+toolsGoFloor) < 0 {
		return toolsGoFloor
	}
	return v.GoVersion
}
