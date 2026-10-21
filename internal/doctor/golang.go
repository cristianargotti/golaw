package doctor

import (
	"fmt"
	"go/version"
)

const (
	idGoToolchain = "doctor.go-toolchain"
	goOlder       = "older"
)

// GoFacts separates collected evidence from assessment so doctor never changes a module.
type GoFacts struct {
	Local string `json:"local"`
	Line  string `json:"line"`
	Tools string `json:"tools"`
	Error string `json:"error"`
}

// AssessGo describes the required toolchain without changing the go line or arming a rung.
func AssessGo(facts GoFacts) []Line {
	line := Line{ID: idGoToolchain, Key: "go", State: stateUnknown, Code: "go-unreadable"}
	switch {
	case facts.Error != "":
		line.Detail = facts.Error
	case !version.IsValid(facts.Local) || !version.IsValid("go"+facts.Line):
		line.Detail = fmt.Sprintf("cannot compare local %q with the go line %q of go.mod", facts.Local, facts.Line)
	case version.Compare(facts.Local, "go"+facts.Line) < 0:
		line.State, line.Code = goOlder, "go-older-than-line"
		line.Detail = fmt.Sprintf("local %s is older than the go line %s of go.mod, and golaw.mk exports GOTOOLCHAIN=local, so every Go target stops: install go%s, the toolchain the go line names for CI and the pipeline image, or run make with GOTOOLCHAIN=go%s",
			facts.Local, facts.Line, facts.Line, facts.Line)
	case facts.Tools != "" && !version.IsValid("go"+facts.Tools):
		line.Detail = fmt.Sprintf("cannot compare local %q with the go line %q of tools/go.mod", facts.Local, facts.Tools)
	case facts.Tools != "" && version.Compare(facts.Local, "go"+facts.Tools) < 0:
		line.State, line.Code = goOlder, "go-older-than-tools"
		line.Detail = fmt.Sprintf("local %s is older than the go line %s of tools/go.mod, which golaw setup builds the pinned tools with: install go%s or newer before golaw setup",
			facts.Local, facts.Tools, facts.Tools)
	default:
		line.State, line.Code = stateOK, "go-meets-line"
		line.Detail = fmt.Sprintf("local %s meets the go line %s of go.mod; CI runs the go line's toolchain through setup-go with go-version-file: go.mod", facts.Local, facts.Line)
	}
	return []Line{line}
}
