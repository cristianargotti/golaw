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

// TestInstallLimitsClearByTheirRewrite pins the rewrites the readInstalls
// row of docs/rules.md names: each form is a stated limit, and its rewrite
// within the subset reads the packages it installs, none for a command
// that only names an install; every option the row lists after the verb is
// one the reader reads.
func TestInstallLimitsClearByTheirRewrite(t *testing.T) {
	t.Parallel()
	cases := []struct{ limit, rewrite, want string }{
		{"RUN DEBIAN_FRONTEND=noninteractive apt-get install -y git=1", "ENV DEBIAN_FRONTEND=noninteractive\nRUN apt-get install -y git=1", "git"},
		{"RUN if true; then apk add git=1; fi", "RUN apk add git=1", "git"},
		{"RUN (apk add git=1) && sudo apk add tzdata=2 && /sbin/apk add curl=3", "RUN apk add git=1 && apk add tzdata=2 && apk add curl=3", "git tzdata curl"},
		{"RUN apt-get install --allow-downgrades -y git=1", "RUN apt-get --allow-downgrades install -y git=1", "git"},
		{"RUN apt-get -o Acquire::Retries=3 install -y git=1", "RUN apt-get -o=Acquire::Retries=3 install -y git=1", "git"},
		{"RUN apt-get install -y -t bookworm-backports git=1", "RUN apt-get --target-release=bookworm-backports install -y git=1", "git"},
		{"RUN apk add --no-cache --virtual .deps git=1", "RUN apk add --no-cache git=1", "git"},
		{"RUN apk add $PACKAGES \"tzdata\"", "RUN apk add git=1 tzdata", "git tzdata"},
		{"RUN apt-get install -y git=1 | tee /tmp/log", "RUN apt-get install -y git=1", "git"},
		{"RUN apk add git=1 || true", "RUN apk add git=1", "git"},
		{"RUN [\"apk\", \"add\", \"git=1\"]", "RUN apk add git=1", "git"},
		{"RUN <<EOF\napk add git=1\nEOF", "RUN apk add git=1", "git"},
		{"RUN echo apk add is how you install", "RUN echo \"apk add is how you install\"", ""},
	}
	const file = "images/app.Dockerfile"
	for _, tc := range cases {
		_, err := readInstalls(file, []byte("FROM alpine\n"+tc.limit+"\n"))
		if !errors.Is(err, model.ErrOutsideSubset) || !strings.HasPrefix(err.Error(), file+":2: ") {
			t.Errorf("%q: %v, want a stated limit naming %s:2", tc.limit, err, file)
		}
		got, err := readInstalls(file, []byte("FROM alpine\n"+tc.rewrite+"\n"))
		names := make([]string, 0, len(got.installs))
		for _, p := range got.installs {
			names = append(names, p.name)
		}
		if err != nil || strings.Join(names, " ") != tc.want {
			t.Errorf("%q: %v, %v, want %q", tc.rewrite, names, err, tc.want)
		}
	}
	listed := "--no-cache --update -U --upgrade -u --quiet -q -qq --no-progress -y --yes --assume-yes --no-install-recommends --no-install-suggests"
	for option := range strings.FieldsSeq(listed) {
		if !installFlag(option) {
			t.Errorf("the documented option %s is not read", option)
		}
	}
}
