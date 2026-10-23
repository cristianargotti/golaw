package policyfiles

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestReadInstallsReadsTheDeclaredSubset pins the install line reader of
// contract D5: the packages of every install command of a shell form RUN
// with their versions and the line of the instruction, the comment lines
// outside heredoc bodies, and the stated limits of its own.
func TestReadInstallsReadsTheDeclaredSubset(t *testing.T) {
	t.Parallel()
	data := "# syntax=docker/dockerfile:1\nFROM alpine AS ci\n# lint ignore=DL3018\n" +
		"RUN apk add --no-cache git=2.45.2-r0 \\\n  tzdata && apt-get -y install curl\n" +
		"RUN <<EOF\n# not a Dockerfile comment\nEOF\n"
	got, err := readInstalls("Dockerfile", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	want := []installed{{name: "git", version: "2.45.2-r0", line: 4}, {name: "tzdata", line: 4}, {name: "curl", line: 4}}
	if !slices.Equal(got.installs, want) {
		t.Errorf("installs = %+v, want %+v", got.installs, want)
	}
	comments := []dockerComment{{text: "# syntax=docker/dockerfile:1", line: 1}, {text: "# lint ignore=DL3018", line: 3}}
	if !slices.Equal(got.comments, comments) {
		t.Errorf("comments = %+v, want %+v", got.comments, comments)
	}
	for name, text := range map[string]string{
		"unfinished continuation": "FROM alpine\nRUN apk add git \\",
		"unterminated heredoc":    "FROM alpine\nRUN <<EOF\napk add git\n",
	} {
		if _, err := readInstalls("Dockerfile", []byte(text)); !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), "Dockerfile:2:") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestInstallArgumentsFindsTheVerbAfterOptions pins which commands install:
// the verb of the manager after its options, nothing for another command.
func TestInstallArgumentsFindsTheVerbAfterOptions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		words []string
		want  []string
		ok    bool
	}{
		{words: nil},
		{words: []string{"apk", "-U"}},
		{words: []string{"apt-get", "update"}},
		{words: []string{"go", "install", "x"}},
		{words: []string{"apk", "add", "git"}, want: []string{"git"}, ok: true},
		{words: []string{"apt-get", "-qq", "install", "-y", "git"}, want: []string{"-y", "git"}, ok: true},
	}
	for _, tc := range cases {
		got, ok := installArguments(tc.words)
		if ok != tc.ok || ok && !slices.Equal(got, tc.want) {
			t.Errorf("installArguments(%v) = %v, %v", tc.words, got, ok)
		}
	}
}
