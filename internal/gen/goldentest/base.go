package goldentest

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw"
	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/runner"
)

// The ruleguard dsl the rendered go.mod requires, with the go.sum lines of
// the kit's own module graph, so the scratch module resolves it from the
// module cache without the network.
const (
	dslModule  = "github.com/quasilyte/go-ruleguard/dsl"
	dslVersion = "v0.3.23"
	dslSums    = dslModule + " " + dslVersion + " h1:lxjt5B6ZCiBeeNO8/oQsegE6fLeCzuMRoVWSkXC4uvY=\n" +
		dslModule + " " + dslVersion + "/go.mod h1:KeCP03KrjuSO0H1kTuZQCWlQPulDV6YMIXmpQss17rU=\n"
)

// The files of the base the harness reads by name.
const (
	goModFile  = "go.mod"
	goSumFile  = "go.sum"
	policyFile = "golaw.yml"
)

// compiledAt is the date the slice's policy is compiled at: a package
// outside cmd never reads the clock, and the date only decides which
// exceptions of a policy have expired.
func compiledAt() time.Time {
	return time.Date(2026, time.October, 29, 0, 0, 0, 0, time.UTC)
}

// compile answers the effective policy of the slice's golaw.yml.
func compile(data []byte) (*model.EffectivePolicy, error) {
	f, err := policy.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("golaw.yml of the slice: %w", err)
	}
	p, err := policy.Compile(f, compiledAt(), nil)
	if err != nil {
		return nil, fmt.Errorf("golaw.yml of the slice: %w", err)
	}
	return p, nil
}

// baseFiles is the init rendering of every core template for the slice's
// capabilities beside the slice's base, whose files win, so the scratch
// module carries the lint asset and the floors an enrolled repository has.
func baseFiles(s Slice, p *model.EffectivePolicy) (map[string][]byte, error) {
	rendered, err := assets.RenderAllContext(golaw.FS, assets.Context{
		ModulePath: s.Module, GoVersion: goLine(s.Base[goModFile]), KitVersion: p.KitVersion,
		Language: p.Language, Capabilities: p.Capabilities, Decisions: p.Layout.Decisions,
		Main: p.Layout.Main, Tools: tools(), Generate: p.Generate, CIBackend: "none",
	})
	if err != nil {
		return nil, fmt.Errorf("render the base: %w", err)
	}
	files := maps.Clone(s.Base)
	for i := range rendered {
		if _, ok := files[rendered[i].Path]; !ok {
			files[rendered[i].Path] = rendered[i].Data
		}
	}
	for i := range rendered {
		if rendered[i].Path == goModFile {
			module := bytes.TrimRight(s.Base[goModFile], "\n")
			files[goModFile] = slices.Concat(module, []byte("\n\n"), rendered[i].Data)
		}
	}
	if _, ok := files[goSumFile]; !ok {
		files[goSumFile] = []byte(dslSums)
	}
	return files, nil
}

// goLine answers the version of the go directive of a go.mod.
func goLine(gomod []byte) string {
	for line := range strings.SplitSeq(string(gomod), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	return ""
}

// tools maps every pinned tool to its version, as init renders them.
func tools() map[string]string {
	out := map[string]string{}
	for _, tool := range slices.Concat(runner.Toolchain(), runner.Generators()) {
		out[tool.Name] = tool.Version
	}
	return out
}

// start commits the base of a slice into a fresh repository.
func start(tb testing.TB, s Slice) (*harness, error) {
	tb.Helper()
	p, err := compile(s.Base[policyFile])
	if err != nil {
		return nil, err
	}
	files, err := baseFiles(s, p)
	if err != nil {
		return nil, err
	}
	repo := gentest.Module(tb, files)
	h := &harness{tb: tb, s: s, repo: repo, base: repo.Head(), policy: p, work: tb.TempDir()}
	h.report = Report{Dir: repo.Root, Files: map[string][]byte{}, Skipped: map[Step]string{}}
	return h, nil
}
