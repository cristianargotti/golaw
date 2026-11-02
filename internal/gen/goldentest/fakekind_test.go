package goldentest_test

import (
	"context"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genheader"
)

// greeter is the kind of the harness's own tests: a thin regenerable type
// and constructor with their test, one owned method with a hole and its red
// test. extra is appended to the regenerable file for a red row, with TYPE
// standing for the type name.
type greeter struct {
	extra string
	// docInRegenerable puts the package comment in the regenerable file
	// alone, the shape the adopter's generated: lax hides from revive.
	docInRegenerable bool
}

func (greeter) Name() string  { return genkind.KindAdapterHTTP }
func (greeter) Template() int { return 1 }

func (g greeter) New(_ context.Context, in genkind.Input, req genkind.Request) (genkind.Output, error) {
	return g.render(in, req.Name, true), nil
}

func (g greeter) Regenerate(_ context.Context, in genkind.Input, entry manifest.Entry) (genkind.Output, error) {
	return g.render(in, genkind.Name{Area: entry.Area(), Type: entry.Type()}, false), nil
}

func (g greeter) render(in genkind.Input, name genkind.Name, owned bool) genkind.Output {
	lower := strings.ToLower(name.Type)
	dir := "internal/adapters/out/" + name.Area + "/"
	header := genheader.Header{Version: in.Version, Kind: genkind.KindAdapterHTTP, Template: 1}.Line() + "\n"
	values := strings.NewReplacer("AREA", name.Area, "TYPE", name.Type, "LOWER", lower, "MODULE", in.Module, "SUPPORT", in.Policy.Support)
	source, method := regenerableSource, ownedSource
	if g.docInRegenerable {
		source, method = "\n"+packageDoc+strings.TrimPrefix(source, "\n"), strings.TrimPrefix(method, packageDoc)
	}
	regenerable := []genkind.File{
		{Path: dir + "zz_generated_" + lower + ".go", Content: []byte(header + values.Replace(source+g.extra))},
		{Path: dir + "zz_generated_" + lower + "_test.go", Content: []byte(header + values.Replace(regenerableTest))},
	}
	ownedFiles := []genkind.File{
		{Path: dir + lower + ".go", Content: []byte(values.Replace(method))},
		{Path: dir + lower + "_test.go", Content: []byte(values.Replace(ownedTest))},
	}
	out := genkind.Output{
		Entry: manifest.Entry{
			Kind: genkind.KindAdapterHTTP, Template: "adapter-http@1", Name: name.String(),
			Regenerable: paths(regenerable), Owned: paths(ownedFiles),
		},
		Regenerable: regenerable,
	}
	if owned {
		out.Owned = ownedFiles
	}
	return out
}

func paths(files []genkind.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

const regenerableSource = `
package AREA

// TYPE greets an account with a fixed prefix.
type TYPE struct {
	prefix string
}

// NewTYPE answers a TYPE that starts each line with prefix.
func NewTYPE(prefix string) *TYPE {
	return &TYPE{prefix: prefix}
}
`

const regenerableTest = `
package AREA_test

import (
	"testing"

	"MODULE/internal/adapters/out/AREA"
)

func TestNewTYPEAnswersAValue(t *testing.T) {
	t.Parallel()
	if AREA.NewTYPE("hello") == nil {
		t.Fatal("NewTYPE answered nil")
	}
}
`

// packageDoc is the package comment the owned file carries.
const packageDoc = "// Package AREA greets an account.\n"

const ownedSource = packageDoc + `package AREA

import (
	"context"

	"MODULE/SUPPORT"
)

// Greet answers the greeting line of one name.
func (g *TYPE) Greet(ctx context.Context, name string) (string, error) {
	// golaw:hole LOWER.greet join the prefix and the name with one space
	return "", gensupport.Hole("LOWER.greet", ctx, name)
}
`

const ownedTest = `package AREA_test

import (
	"errors"
	"testing"

	"MODULE/internal/adapters/out/AREA"
	"MODULE/SUPPORT"
)

func TestGreetHole(t *testing.T) {
	t.Parallel()
	_, err := AREA.NewTYPE("hello").Greet(t.Context(), "ana")
	if errors.Is(err, gensupport.ErrNotImplemented) {
		t.Fatal("hole LOWER.greet is open: join the prefix and the name with one space")
	}
}
`

// The red extras: a function past go:S107, a regenerable line no generated
// test reaches, and one body copied into both instances.
const (
	eightParameters = `
// Sum adds eight parts.
func Sum(a, b, c, d, e, f, g, h int) int {
	return a + b + c + d + e + f + g + h
}
`
	uncovered = `
// Prefix answers the prefix of the TYPE.
func (g *TYPE) Prefix() string {
	return g.prefix
}
`
	copied = `
// Score scores the values of one account.
func Score(values []int) (int, bool) {
	if len(values) > 100 {
		return len(values), false
	}
	total := 0
	for index, value := range values {
		if value < 0 {
			return index, false
		}
		if value > 1000 {
			return value - 1000, false
		}
		total += value * 2
		total -= index
	}
	if total > 5000 {
		return total - 5000, false
	}
	average := total / len(values)
	if average == 0 {
		return 0, false
	}
	return average + total, true
}
`
)
