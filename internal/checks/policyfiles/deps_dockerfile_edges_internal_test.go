package policyfiles

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestDockerfileContinuedAliases(t *testing.T) {
	t.Parallel()
	digest := "sha256:" + strings.Repeat("a", 64)
	pin := "alpine@" + digest
	for _, row := range []struct {
		name, escape, skipped, ending string
		last                          int
	}{
		{"indented alias", "\\", "", "\n", 3},
		{"trailing spaces and tabs", "\\ \t", "", "\n", 3},
		{"comments and blank lines", "\\\t ", "  # alias below\n \t\n", "\n", 5},
		{"converted line endings", "\\ \t", "\t# alias below\r\n\r\n", "\r\n", 5},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			data := "FROM " + pin + row.escape + row.ending + row.skipped +
				"    AS build" + row.ending + "FROM build" + row.ending
			got, err := readDockerfile("Dockerfile", []byte(data))
			want := []dockerStage{{from: pin, digest: digest, name: "build", line: 1}, {from: "build", line: row.last}}
			if err != nil || !reflect.DeepEqual(got.stages, want) {
				t.Fatalf("stages = %+v, error = %v, want %+v", got.stages, err, want)
			}
		})
	}
}

func TestDockerfileContinuedEnvironmentWhitespace(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, text, value string }{
		{"quoted indentation", "ENV NOTE=\"first\\\n    second\"", "first    second"},
		{"two escape characters", "ENV NOTE=path\\\\ \t\nsuffix", "pathsuffix"},
		{"continuation chain", "ENV NOTE=\"first\\ \t\n    second\\\t\n\tthird\"", "first    second\tthird"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := readDockerfile("Dockerfile", []byte("FROM scratch\n"+row.text+"\n"))
			want := []dockerStage{{from: "scratch", line: 1, env: []dockerVar{{name: "NOTE", value: row.value, line: 2}}}}
			if err != nil || !reflect.DeepEqual(got.stages, want) {
				t.Fatalf("stages = %+v, error = %v, want %+v", got.stages, err, want)
			}
		})
	}
}

func TestDockerfileContinuationTrailingWhitespace(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		text string
		want bool
	}{
		{"FROM scratch\\ \t", true},
		{"ENV NOTE=path\\\\\t ", true},
		{"FROM scratch\\\v", false},
		{"FROM scratch\\\u00a0", false},
		{"FROM scratch \t", false},
	} {
		if got := dockerContinued(row.text); got != row.want {
			t.Errorf("dockerContinued(%q) = %v, want %v", row.text, got, row.want)
		}
	}
}

func TestDockerfileOpaqueShellCommentQuotes(t *testing.T) {
	t.Parallel()
	for _, instruction := range []string{
		"RUN make build # it's needed",
		"RUN make build # \"needed",
		"ONBUILD RUN make build # it's needed",
		"COPY file /dest # it's needed",
		"ADD file /dest # it's needed",
	} {
		t.Run(instruction, func(t *testing.T) {
			t.Parallel()
			data := "FROM scratch AS build\n" + instruction + "\nFROM build\n"
			got, err := readDockerfile("Dockerfile", []byte(data))
			want := []dockerStage{{from: "scratch", name: "build", line: 1}, {from: "build", line: 3}}
			if err != nil || !reflect.DeepEqual(got.stages, want) {
				t.Fatalf("stages = %+v, error = %v, want %+v", got.stages, err, want)
			}
		})
	}
}

func TestDockerfileUnclosedQuotesWithHeredocMarker(t *testing.T) {
	t.Parallel()
	for _, instruction := range []string{
		"RUN cat <<EOF # it's needed",
		"RUN echo 'unterminated <<EOF",
		"RUN echo \"<<EOF\" # it's needed",
		"ONBUILD RUN cat <<EOF # it's needed",
	} {
		t.Run(instruction, func(t *testing.T) {
			t.Parallel()
			data := "FROM scratch\n" + instruction + "\nFROM unpinned\nEOF\n"
			got, err := readDockerfile("deploy/Dockerfile", []byte(data))
			if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), "deploy/Dockerfile:2:") ||
				!strings.Contains(err.Error(), "heredoc") || len(got.stages) != 0 {
				t.Fatalf("document = %+v, error = %v, want a heredoc limit at line 2", got, err)
			}
		})
	}
}
