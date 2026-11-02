package goldentest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/gen/imports"
)

// twoHoles is a file whose first hole a fill already closed with fewer
// lines, so the second hole sits above the line the commands left it on.
const twoHoles = `package p

func A() int {
	return 1
}

func B() int {
	// golaw:hole p.b answer two
	return 0
}
`

func TestFilledFindsAHoleMovedByAnEarlierFill(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(file, []byte(twoHoles), fillMode); err != nil {
		t.Fatal(err)
	}
	// The line the hole had before the fill of p.a shrank A by two lines.
	stale := holes.Hole{ID: "p.b", Path: "p.go", Line: 12}
	fix, err := filled(file, &Fill{ID: "p.b", Body: "\treturn 2"}, stale, imports.Std())
	if err != nil {
		t.Fatal(err)
	}
	if got := string(fix.Content); !strings.Contains(got, "func B() int {\n\treturn 2\n}") || strings.Contains(got, "golaw:hole") || !strings.Contains(got, "func A() int {\n\treturn 1\n}") {
		t.Errorf("filled = %s", got)
	}
}

func TestFilledRefusesAHoleTheFileNoLongerHolds(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(file, []byte("package p\n\nfunc A() int {\n\treturn 1\n}\n"), fillMode); err != nil {
		t.Fatal(err)
	}
	if _, err := filled(file, &Fill{ID: "p.b", Body: "return 2"}, holes.Hole{ID: "p.b", Path: "p.go", Line: 4}, imports.Std()); err == nil || !strings.Contains(err.Error(), "p.b") {
		t.Errorf("filled over a file without its hole = %v, want an error naming p.b", err)
	}
}

func TestFilledFormatsTheBodyAsTheHookWould(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(file, []byte(twoHoles), fillMode); err != nil {
		t.Fatal(err)
	}
	fix, err := filled(file, &Fill{ID: "p.b", Body: "if true {\nreturn 2\n}\nreturn 3"}, holes.Hole{ID: "p.b", Path: "p.go", Line: 8}, imports.Std())
	if err != nil {
		t.Fatal(err)
	}
	if got := string(fix.Content); !strings.Contains(got, "func B() int {\n\tif true {\n\t\treturn 2\n\t}\n\treturn 3\n}") {
		t.Errorf("filled = %s", got)
	}
}

// ownedHoles is an owned file with two holes, so the fill of the first moves
// the line of the second.
const ownedHoles = "package p\n\n" +
	"// First answers one.\nfunc First() int {\n\t// golaw:hole p.first answer one\n\tpanic(\"golaw: fill p.first\")\n}\n\n" +
	"// Second answers two.\nfunc Second() int {\n\t// golaw:hole p.second answer two\n\tpanic(\"golaw: fill p.second\")\n}\n"

func TestFilledFindsTheHoleWhereTheFileHoldsItNow(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(file, []byte(ownedHoles), fillMode); err != nil {
		t.Fatal(err)
	}
	found, err := holes.Find("p/p.go", []byte(ownedHoles))
	if err != nil || len(found) != 2 {
		t.Fatalf("holes.Find = %v, %v", found, err)
	}
	bodies := []string{"\tone := 1\n\tif one > 0 {\n\t\treturn one\n\t}\n\treturn 0\n", "\treturn 2\n"}
	for i, hole := range found {
		fix, fillErr := filled(file, &Fill{ID: hole.ID, Body: bodies[i]}, hole, imports.Std())
		if fillErr != nil {
			t.Fatalf("fill %s at its line before the first fill: %v", hole.ID, fillErr)
		}
		if writeErr := os.WriteFile(file, fix.Content, fillMode); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(content); strings.Contains(got, "golaw:hole") || !strings.Contains(got, "\treturn 2\n") || !strings.Contains(got, "\t\treturn one\n") {
		t.Errorf("filled file =\n%s", got)
	}
}

func TestFilledNamesTheHoleTheFileNoLongerHolds(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(file, []byte("package p\n\nfunc F() {}\n"), fillMode); err != nil {
		t.Fatal(err)
	}
	hole := holes.Hole{ID: "p.gone", Path: "p/p.go", Line: 3}
	if _, err := filled(file, &Fill{ID: hole.ID, Body: "\treturn\n"}, hole, imports.Std()); err == nil || !strings.Contains(err.Error(), "p.gone") {
		t.Errorf("filled of a hole the file no longer holds = %v", err)
	}
}

// casesHoles is an owned file with two holes, as a port's cases.go holds
// one per method.
const casesHoles = `package p

func one() int {
	// golaw:hole p.one
	return 0
}

func two() int {
	// golaw:hole p.two
	return 0
}
`

// TestFilledFindsAHoleWhereTheEarlierFillMovedIt fills the first hole of a
// file with a body longer than the stub and an import, then the second
// through the hole the commands recorded, whose line no longer holds it.
func TestFilledFindsAHoleWhereTheEarlierFillMovedIt(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "cases.go")
	if err := os.WriteFile(file, []byte(casesHoles), fillMode); err != nil {
		t.Fatal(err)
	}
	recorded, err := holes.Find("p/cases.go", []byte(casesHoles))
	if err != nil || len(recorded) != 2 {
		t.Fatalf("holes.Find = %v, %v", recorded, err)
	}
	fills := []Fill{
		{ID: "p.one", Body: "\tn := 1\n\tn *= 3\n\tn += 2\n\treturn len(strings.Repeat(\"x\", n))\n", Imports: []genkind.Import{{Path: "strings"}}},
		{ID: "p.two", Body: "\treturn 2\n"},
	}
	for i := range fills {
		fix, fillErr := filled(file, &fills[i], recorded[i], imports.Std())
		if fillErr != nil {
			t.Fatalf("fill %s: %v", fills[i].ID, fillErr)
		}
		if writeErr := os.WriteFile(file, fix.Content, fillMode); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); strings.Contains(got, holes.Marker) || !strings.Contains(got, "return 2") || !strings.Contains(got, `"strings"`) {
		t.Errorf("the filled file keeps a hole or lost a fill:\n%s", got)
	}
}

// TestFilledRefusesAHoleItsCasesFileLost names the hole a fill cannot
// find any more, because the file it was recorded in changed under it.
func TestFilledRefusesAHoleItsCasesFileLost(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "cases.go")
	if err := os.WriteFile(file, []byte("package p\n\nfunc one() int {\n\treturn 1\n}\n"), fillMode); err != nil {
		t.Fatal(err)
	}
	hole := holes.Hole{ID: "p.one", Path: "p/cases.go", Line: 4}
	if _, err := filled(file, &Fill{ID: "p.one", Body: "\treturn 2\n"}, hole, imports.Std()); err == nil || !strings.Contains(err.Error(), "p.one") {
		t.Errorf("filled over a file without its hole = %v, want an error naming p.one", err)
	}
}
