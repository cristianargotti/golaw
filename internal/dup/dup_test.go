package dup_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/dup"
)

// body is one function body of 107 tokens over 24 lines, the shape a
// method copied into two adapters has.
const body = `(values []int) (int, bool) {
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

// source is a file of package pkg whose function name carries text:
// line 1 is the package clause, line 3 the comment, line 4 the signature.
func source(pkg, name, text string) []byte {
	return []byte("package " + pkg + "\n\n// " + name + " scores values.\nfunc " + name + text)
}

func TestMeasureFindsACopiedBody(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"a.go": source("a", "Score", body), "b.go": source("b", "Grade", body)}
	got, err := dup.Measure(files, nil, dup.Default())
	if err != nil {
		t.Fatal(err)
	}
	want := dup.Report{Duplicated: 48, New: 50, Blocks: []dup.Block{{
		Tokens: 107,
		Spans:  []dup.Span{{Path: "a.go", Start: 4, End: 27}, {Path: "b.go", Start: 4, End: 27}},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Measure = %+v, want %+v", got, want)
	}
	if got.Percent() != 96 {
		t.Errorf("Percent = %v, want 96", got.Percent())
	}
	again, err := dup.Measure(files, nil, dup.Default())
	if err != nil || !reflect.DeepEqual(again, got) {
		t.Errorf("a second measure differs: %+v, %v", again, err)
	}
}

func TestMeasureRedAndNearShapes(t *testing.T) {
	t.Parallel()
	renamed := strings.ReplaceAll(body, "total", "sum")
	commented := strings.Replace(body, "\ttotal := 0\n", "\t// the running total of the values\n\ttotal := 0\n", 1)
	literals := strings.NewReplacer("1000", "2000", "5000", "6000").Replace(body)
	packed := strings.ReplaceAll(body, "\n", " ; ")
	shared := "(values []int) (int, bool) {\n\treturn grade(values)\n}\n"
	tests := map[string]struct {
		files map[string][]byte
		dup   bool
	}{
		"red: literals that differ only in value still match": {
			files: map[string][]byte{"a.go": source("a", "Score", body), "b.go": source("b", "Grade", literals)}, dup: true,
		},
		"red: a comment inside one copy does not hide it": {
			files: map[string][]byte{"a.go": source("a", "Score", body), "b.go": source("b", "Grade", commented)}, dup: true,
		},
		"red: a third copy joins the block": {
			files: map[string][]byte{"a.go": source("a", "Score", body), "b.go": source("b", "Grade", body), "c.go": source("c", "Rank", body)}, dup: true,
		},
		"near: both instances call one shared function": {
			files: map[string][]byte{
				"a.go": source("a", "Score", shared), "b.go": source("b", "Grade", shared), "c.go": source("c", "grade", body),
			},
		},
		"near: renamed identifiers break the match": {
			files: map[string][]byte{"a.go": source("a", "Score", body), "b.go": source("b", "Grade", renamed)},
		},
		"near: a copy packed on fewer than ten lines": {
			files: map[string][]byte{"a.go": source("a", "Score", packed), "b.go": source("b", "Grade", packed)},
		},
		"near: a file that is not Go is never read": {
			files: map[string][]byte{"a.go": source("a", "Score", body), "notes.txt": source("b", "Grade", body)},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := dup.Measure(tc.files, nil, dup.Default())
			if err != nil {
				t.Fatal(err)
			}
			if (got.Duplicated > 0) != tc.dup || (len(got.Blocks) == 1) != tc.dup {
				t.Fatalf("Measure = %+v, want duplicated %v", got, tc.dup)
			}
		})
	}
}

func TestMeasureCountsOnlyTheNewLines(t *testing.T) {
	t.Parallel()
	files := map[string][]byte{"a.go": source("a", "Score", body), "b.go": source("b", "Grade", body)}
	lines := make([]int, 0, 27)
	for line := 1; line <= 27; line++ {
		lines = append(lines, line)
	}
	got, err := dup.Measure(files, map[string][]int{"b.go": lines}, dup.Default())
	if err != nil {
		t.Fatal(err)
	}
	if got.Duplicated != 24 || got.New != 25 {
		t.Fatalf("Measure = %+v, want 24 duplicated of 25 new lines", got)
	}
	none, err := dup.Measure(files, map[string][]int{}, dup.Default())
	if err != nil || none.New != 0 || none.Percent() != 0 {
		t.Fatalf("no new line = %+v, %v; want 0 new and 0 percent", none, err)
	}
}

func TestMeasureRefusesWhatItCannotMeasure(t *testing.T) {
	t.Parallel()
	if _, err := dup.Measure(nil, nil, dup.Options{}); !errors.Is(err, dup.ErrOptions) {
		t.Errorf("zero options = %v, want ErrOptions", err)
	}
	broken := map[string][]byte{"a.go": []byte("package a\n\nvar s = \"unterminated\n")}
	if _, err := dup.Measure(broken, nil, dup.Default()); !errors.Is(err, dup.ErrTokenize) || !strings.Contains(err.Error(), "a.go") {
		t.Errorf("broken source = %v, want ErrTokenize naming a.go", err)
	}
	if got := (dup.Report{}).Percent(); got != 0 {
		t.Errorf("empty Percent = %v", got)
	}
	if got := dup.Default(); got.MinTokens != 100 || got.MinLines != 10 {
		t.Errorf("Default = %+v", got)
	}
}
