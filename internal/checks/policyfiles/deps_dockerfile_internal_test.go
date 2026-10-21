package policyfiles

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestDockerfilePublishesStagesAndEnvironment(t *testing.T) {
	t.Parallel()
	pin := "alpine@sha256:" + strings.Repeat("a", 64)
	data := "ARG BASE=" + pin + "\n" +
		"FROM ${BASE} AS build\n" +
		"ENV OTHER=value GOTOOLCHAIN=\"local\"\n" +
		"FROM build AS final\n" +
		"ENV GOTOOLCHAIN auto\n"
	got, err := readDockerfile("Dockerfile", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if got.path != "Dockerfile" || len(got.stages) != 2 {
		t.Fatalf("document = %+v", got)
	}
	first, last := got.stages[0], got.stages[1]
	if first.from != pin || first.digest != "sha256:"+strings.Repeat("a", 64) ||
		first.name != "build" || first.line != 2 {
		t.Fatalf("first stage = %+v", first)
	}
	if len(first.env) != 2 || first.env[1] != (dockerVar{
		name: "GOTOOLCHAIN", value: "local", line: 3,
	}) {
		t.Fatalf("first environment = %+v", first.env)
	}
	if last.from != "build" || last.name != "final" ||
		len(last.env) != 1 || last.env[0] != (dockerVar{
		name: "GOTOOLCHAIN", value: "auto", line: 5,
	}) {
		t.Fatalf("final stage environment = %+v", last)
	}
}

func TestDockerfileReadsConvertedLineEndings(t *testing.T) {
	t.Parallel()
	pin := "alpine@sha256:" + strings.Repeat("a", 64)
	got, err := readDockerfile("Dockerfile", []byte("FROM "+pin+" AS build\r\nENV GOTOOLCHAIN=local\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.stages) != 1 || got.stages[0].from != pin || got.stages[0].name != "build" ||
		len(got.stages[0].env) != 1 || got.stages[0].env[0].value != "local" {
		t.Fatalf("converted line endings = %+v", got)
	}
}

func TestDockerfileLiteralInstructions(t *testing.T) {
	t.Parallel()
	pin := "alpine@sha256:" + strings.Repeat("A", 64)
	data := "# syntax=docker/dockerfile:1\n# escape=\\\n# check=skip=all\n" +
		"ARG REGISTRY=alpine\nARG BASE=${REGISTRY}@sha256:" + strings.Repeat("A", 32) + "\\\n" +
		"# digest continues below\n\n" + strings.Repeat("A", 32) + "\n" +
		"from --platform=${TARGETPLATFORM} $BASE as build\n" +
		"ARG BASE=busybox\nENV OTHER='two words' ESCAPED=two\\ words GOTOOLCHAIN=${TOOLCHAIN:-local}\n" +
		"FROM ${BASE}\nENV GOTOOLCHAIN 'local'\nLABEL ignored=value\n"
	got, err := readDockerfile("deploy/Dockerfile", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.stages) != 2 || got.stages[0].from != pin || got.stages[1].from != pin ||
		got.stages[0].line != 9 || got.stages[0].digest != "sha256:"+strings.Repeat("A", 64) {
		t.Fatalf("stages = %+v", got.stages)
	}
	values := got.stages[0].env
	if len(values) != 3 || values[0].value != "two words" || values[1].value != "two words" ||
		values[2].value != "${TOOLCHAIN:-local}" || values[2].line != 11 {
		t.Fatalf("environment = %+v", values)
	}
	if len(got.stages[1].env) != 1 || got.stages[1].env[0].value != "local" {
		t.Fatalf("stage-local environment = %+v", got.stages[1].env)
	}
}

func TestDockerfileSkipsLiteralBodies(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"RUN <<EOF\nFROM unpinned\\\n# escape=`\nEOF\n",
		"COPY <<'EOF' /file\nFROM unpinned\nEOF\n",
		"RUN <<-\"EOF\"\n\tFROM unpinned\n\tEOF\n",
		"COPY <<FIRST <<SECOND /files/\nFROM unpinned\nFIRST\nFROM unpinned\nSECOND\n",
		"RUN echo \"<<EOF\"\n",
		"RUN [\"echo\", \"<<EOF\"]\n",
		"RUN cat < /input\n",
		"RUN cat <<< \"$value\"\n",
		"RUN echo $((1<<3))\n",
		"RUN cat << EOF\n",
		"RUN cat 3<<E.O.F\nFROM unpinned\nE.O.F\n",
		"RUN cat <<'END DOC'\nFROM unpinned\nEND DOC\n",
		"RUN cat <<''\nFROM unpinned\n\n",
		"RUN cat <<'$EOF'\nFROM unpinned\n$EOF\n",
		"RUN cat <<\\$EOF\nFROM unpinned\n$EOF\n",
		"RUN cat <<123\nFROM unpinned\n123\n",
		"ADD <<EOF /file\nFROM unpinned\nEOF\n",
		"ONBUILD RUN <<EOF\nFROM unpinned\nEOF\n",
		"ONBUILD COPY <<EOF /file\nFROM unpinned\nEOF\n",
		"ONBUILD ADD <<EOF /file\nFROM unpinned\nEOF\n",
	} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			got, err := readDockerfile("Dockerfile", []byte("FROM scratch AS build\n"+body+"FROM build\n"))
			if err != nil || len(got.stages) != 2 || got.stages[1].from != "build" {
				t.Fatalf("document = %+v, error = %v", got, err)
			}
		})
	}
}

func TestDockerfileNamesLimits(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, data, reason string
	}{
		{"escape directive", "# escape=`\nFROM scratch\n", "escape parser directive"},
		{"syntax directive", "# syntax=example.test/custom\nFROM scratch\n", "syntax parser directive"},
		{"continuation", "FROM scratch\\", "backslash continuation"},
		{"quotes", "FROM \"alpine\n", "quoting in FROM"},
		{"missing image", "FROM\n", "FROM instruction"},
		{"unknown flag", "FROM --other\n", "FROM flag"},
		{"bad alias", "FROM scratch extra name\n", "FROM instruction"},
		{"missing argument", "FROM $MISSING\n", "unresolved argument"},
		{"empty argument", "ARG BASE=\nFROM $BASE\n", "unresolved argument"},
		{"spaced argument", "ARG BASE='two words'\nFROM $BASE\n", "unresolved argument"},
		{"unresolved default", "ARG BASE=$MISSING\nFROM $BASE\n", "unresolved argument"},
		{"missing brace", "FROM ${BASE\n", "parameter expression"},
		{"parameter operator", "FROM ${BASE:-alpine}\n", "parameter expression"},
		{"non-name parameter", "FROM $1\n", "parameter expression"},
		{"trailing dollar", "FROM alpine$\n", "parameter expression"},
		{"missing delimiter", "RUN <<EOF\nFROM scratch\n", "heredoc"},
		{"computed delimiter", "RUN <<$EOF\nFROM scratch\n", "heredoc"},
		{"partly quoted computed delimiter", "RUN <<'prefix'$EOF''\nbody\nprefix$EOF\nFROM scratch\n", "heredoc"},
		{"undeclared argument", "ARG A B\nFROM $A\n", "deploy/Dockerfile:2:"},
		{"invalid argument referenced", "ARG invalid-name=value\nFROM ${invalid-name}\n", "deploy/Dockerfile:2:"},
		{"quoted delimiter incomplete", "RUN <<'EOF\nFROM scratch\n", "heredoc"},
		{"escaped image parameter", "ARG BASE=scratch\nFROM \\$BASE\n", "escaped parameter"},
		{"escaped default parameter", "ARG BASE=scratch\nARG NEXT=\\$BASE\nFROM $NEXT\n", "unresolved argument"},
		{"quoted default parameter", "ARG BASE=scratch\nARG NEXT='$BASE'\nFROM $NEXT\n", "unresolved argument"},
		{"missing environment value", "FROM scratch\nENV GOTOOLCHAIN\n", "ENV without a value"},
		{"empty environment", "FROM scratch\nENV\n", "ENV without a value"},
		{"mixed environment", "FROM scratch\nENV OTHER=x GOTOOLCHAIN\n", "ENV assignment"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := readDockerfile("deploy/Dockerfile", []byte(row.data))
			if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), row.reason) ||
				!strings.Contains(err.Error(), "deploy/Dockerfile:") || len(got.stages) != 0 {
				t.Fatalf("document = %+v, error = %v, want limit naming %s", got, err, row.reason)
			}
		})
	}
}

func TestDockerfileLiteralDigestValidation(t *testing.T) {
	t.Parallel()
	for _, digest := range []string{
		"", "sha256:abcd", "sha256:" + strings.Repeat("g", 64),
		"sha256:" + strings.Repeat("a", 65), "sha512:" + strings.Repeat("a", 64),
		"sha256:" + strings.Repeat("a", 64) + "@extra",
	} {
		t.Run(digest, func(t *testing.T) {
			t.Parallel()
			got, err := readDockerfile("Dockerfile", []byte("FROM alpine@"+digest+"\n"))
			if err != nil || len(got.stages) != 1 || got.stages[0].digest != "" {
				t.Fatalf("document = %+v, error = %v, want literal without a complete digest", got, err)
			}
		})
	}
}

func TestDockerfileNamesAndOpaqueComments(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		want bool
	}{
		{"Dockerfile", true},
		{"deploy/Dockerfile", true},
		{"Dockerfile.dev", true},
		{"deploy/api.Dockerfile", true},
		{"Dockerfile.txt", false},
		{"Dockerfile.dev.md", false},
		{"Dockerfile.dockerignore", false},
		{"dockerfile", false},
		{"Other", false},
	} {
		if got := isDockerfile(row.name); got != row.want {
			t.Errorf("isDockerfile(%q) = %v, want %v", row.name, got, row.want)
		}
	}
	for _, text := range []string{
		"# regular comment\n# escape=`\nFROM scratch\n",
		"\n# syntax=custom\nFROM scratch\n",
		"FROM scratch\n# escape=`\n",
		"ENV IGNORED=before-stage\nFROM scratch\n",
		"\ufeffFROM scratch\r\n",
		"\ufeff# syntax=docker/dockerfile:1\r\nFROM scratch\r",
		"FROM scratch\nRUN <<EOF\nFROM unpinned\nEOF\r",
	} {
		got, err := readDockerfile("Dockerfile", []byte(text))
		if err != nil || len(got.stages) != 1 || len(got.stages[0].env) != 0 {
			t.Fatalf("document = %+v, error = %v", got, err)
		}
	}
}

func TestDockerfileOfficialFrontends(t *testing.T) {
	t.Parallel()
	for _, frontend := range []string{
		"docker/dockerfile", "docker/dockerfile:1.7", "docker/dockerfile:1.4-labs",
		"docker.io/docker/dockerfile:1", "docker.io/docker/dockerfile:latest",
		"docker/dockerfile:1@sha256:" + strings.Repeat("a", 64),
		"docker.io/docker/dockerfile@sha256:" + strings.Repeat("b", 64),
	} {
		t.Run(frontend, func(t *testing.T) {
			t.Parallel()
			got, err := readDockerfile("Dockerfile", []byte("# syntax="+frontend+"\nFROM scratch\n"))
			if err != nil || len(got.stages) != 1 || got.stages[0].from != "scratch" {
				t.Fatalf("document = %+v, error = %v", got, err)
			}
		})
	}
}

func TestDockerfileGlobalArguments(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, data, from string }{
		{"global argument", "ARG BASE extra\nFROM scratch\n", "scratch"},
		{"unused names", "ARG A B\nFROM scratch\n", "scratch"},
		{"multiple defaults", "ARG REGISTRY=example.test BASE=alpine\nFROM ${REGISTRY}/${BASE}\n", "example.test/alpine"},
		{"earlier default", "ARG REGISTRY=example.test BASE=${REGISTRY}/alpine\nFROM $BASE\n", "example.test/alpine"},
		{"invalid word ignored", "ARG invalid-name=value =value BASE=alpine\nFROM $BASE\n", "alpine"},
		{"unrelated quoted parameter", "ARG OTHER='$UNKNOWN' BASE=alpine\nFROM $BASE\n", "alpine"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := readDockerfile("Dockerfile", []byte(row.data))
			if err != nil || len(got.stages) != 1 || got.stages[0].from != row.from {
				t.Fatalf("document = %+v, error = %v, want %s", got, err, row.from)
			}
		})
	}
}

func TestDockerfilePreservesEscapesAndInstructionBoundaries(t *testing.T) {
	t.Parallel()
	data := "FROM scratch\nENV GOTOOLCHAIN=\"lo\\cal\" OTHER=path\\\\\nsuffix\nFROM alpine:3.22\n"
	got, err := readDockerfile("Dockerfile", []byte(data))
	if err != nil || len(got.stages) != 2 || got.stages[1].from != "alpine:3.22" ||
		got.stages[0].env[0].value != "lo\\cal" || got.stages[0].env[1].value != "pathsuffix" {
		t.Fatalf("document = %+v, error = %v", got, err)
	}
}

func TestDockerfileLeavesPlatformValuesOpaque(t *testing.T) {
	t.Parallel()
	pin := "alpine@sha256:" + strings.Repeat("a", 64)
	for _, platform := range []string{"'linux/amd64'", "'$BUILDPLATFORM'", "\"${TARGETPLATFORM:-linux/amd64}\""} {
		t.Run(platform, func(t *testing.T) {
			t.Parallel()
			data := "ARG BASE=" + pin + "\nFROM --platform=" + platform + " ${BASE}\n"
			got, err := readDockerfile("Dockerfile", []byte(data))
			if err != nil || len(got.stages) != 1 || got.stages[0].from != pin || got.stages[0].digest == "" {
				t.Fatalf("document = %+v, error = %v", got, err)
			}
		})
	}
}

func TestDockerfileContinuationEscape(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		text string
		want bool
	}{
		{"FROM scratch", false},
		{"FROM scratch\\", true},
		{"ENV VALUE=path\\\\", true},
		{"ENV VALUE=path\\\\\\", true},
	} {
		if got := dockerContinued(row.text); got != row.want {
			t.Errorf("dockerContinued(%q) = %v, want %v", row.text, got, row.want)
		}
	}
}
