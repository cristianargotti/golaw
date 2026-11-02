package goldentest

import "github.com/cristianargotti/golaw/internal/gen/genkind"

// Command is one golaw new of a slice, in order.
type Command struct {
	Kind    string
	Request genkind.Request
}

// Fill is the recorded fill of one hole: the replacement of the whole
// function body (the statements between the braces) and the imports it
// needs, the domain lines an agent would write.
type Fill struct {
	ID      string           `yaml:"id"`
	Body    string           `yaml:"body"`
	Imports []genkind.Import `yaml:"imports"`
}

// Slice is one golden slice: a base the commands generate into, the kinds
// they need and the fills that close the holes they open.
type Slice struct {
	Name     string            // the kind under proof, used in messages
	Module   string            // the module path of the base go.mod
	Base     map[string][]byte // the files committed at the base commit: go.mod, golaw.yml, the contracts
	Kinds    []genkind.Kind    // the kinds the commands need, support and wiring included
	Commands []Command
	Fills    []Fill
}

// Step is one gate condition the harness proves over a slice.
type Step string

// The steps, in the order the harness runs them.
const (
	StepBuild    Step = "build"    // go build, go vet, go test in the pipeline shape and with -race
	StepLint     Step = "lint"     // golangci-lint run with the rendered lint asset: zero findings
	StepScan     Step = "scan"     // golaw scan in process: zero enforced findings
	StepSurface  Step = "surface"  // golaw lint --surface under the strict set on the non-Go files of the slice when a surface tool is installed
	StepCoverage Step = "coverage" // coverage on new code against the base commit: at least 80 with the fills, every regenerable line covered with the holes open
	StepDup      Step = "dup"      // internal/dup over the new lines: under 1.5 percent
	StepCheck    Step = "check"    // gen.Check: open holes before the fills, clean after
	StepFormat   Step = "format"   // byte identity with golangci-lint fmt
	StepLaws     Step = "laws"     // the per law assertions of M7.10.2 (Laws)
	StepFloor    Step = "floor"    // go vet at the adopter's floor of A9 (go 1.24) with that toolchain
)

// order answers every step in declaration order, the order a run follows.
func order() []Step {
	return []Step{StepBuild, StepLint, StepScan, StepSurface, StepCoverage, StepDup, StepCheck, StepFormat, StepLaws, StepFloor}
}

// Report is what a run measured, so a kind's golden test can assert more
// than the harness itself requires.
type Report struct {
	Dir         string            // the scratch module root, under the test's temporary directory
	Files       map[string][]byte // every file the commands wrote, by slash path, before the fills
	Coverage    float64           // new code coverage with the fills
	OpenCovered bool              // every regenerable line covered with the holes open
	Duplication float64           // the percent of new lines inside a duplicated block
	Skipped     map[Step]string   // the steps skipped, each with its stated reason
}
