package policyfiles

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestDependencyAgeCommandReadsItsDeclaredSubset pins the recipe reader of
// policy.deps-age-config: the depsage forms it recognizes with the age they
// enforce, the commands it skips because they cannot run the check, and
// the constructs it names as limits, each at the file and line it was given.
func TestDependencyAgeCommandReadsItsDeclaredSubset(t *testing.T) {
	t.Parallel()
	const pinned = "go run github.com/cristianargotti/golaw/tools/depsage@"
	for _, tc := range []struct {
		name, command string
		days          int
		invocation    bool
		limit         string
	}{
		{name: "plain", command: "go run ./tools/depsage", days: 7, invocation: true},
		{name: "double dash", command: "go run ./tools/depsage --days=9", days: 9, invocation: true},
		{name: "allow inline", command: "go run ./tools/depsage -allow=example.test/a@v1.0.0 -days 8", days: 8, invocation: true},
		{name: "allow separate", command: "go run ./tools/depsage -allow example.test/a@v1.0.0", days: 7, invocation: true},
		{name: "continued line", command: "go run ./tools/depsage \\\n-days 12", days: 12, invocation: true},
		{name: "prefixes", command: "+@go run ./tools/depsage -days 7", days: 7, invocation: true},
		{name: "ignored after quiet", command: "@-go run ./tools/depsage -days 7", days: 0, invocation: true},
		{name: "prerelease pin", command: pinned + "v0.0.0-devel -days 7", days: 7, invocation: true},
		{name: "two assignments", command: "GOFLAGS=-mod=mod GOTOOLCHAIN=auto go run ./tools/depsage", days: 7, invocation: true},
		{name: "empty line", command: ""},
		{name: "echo", command: "echo done"},
		{name: "printf", command: "printf '%s\\n' done"},
		{name: "true", command: "true"},
		{name: "go build", command: "go build ./..."},
		{name: "go alone", command: "go"},
		{name: "git", command: "git diff --exit-code"},
		{name: "unknown executable", command: "./scripts/age.sh", limit: "./scripts/age.sh, an executable"},
		{name: "go run of another program", command: "go run ./scripts/age", limit: "go run of another program"},
		{name: "go tool", command: "go tool depsage -days 7", limit: "go tool of another program"},
		{name: "unpinned module", command: pinned + "latest -days 7", limit: "go run of another program"},
		{name: "short pin", command: pinned + "v0.3 -days 7", limit: "go run of another program"},
		{name: "letters in the pin", command: pinned + "v0.3.x -days 7", limit: "go run of another program"},
		{name: "computed pin", command: pinned + "v0.3.0-$(REV) -days 7", limit: "go run of another program"},
		{name: "pipe", command: "go run ./tools/depsage -days 7 | tee age.log", limit: "the shell operator |"},
		{name: "shell variable", command: "go run ./tools/depsage -days $$DAYS", limit: "the shell operator $$"},
		{name: "redirection", command: "go run ./tools/depsage -days 7 > age.log", limit: "the shell operator >"},
		{name: "computed days", command: "go run ./tools/depsage -days=$(DAYS)", limit: "the computed value $(DAYS) of -days"},
		{name: "missing value", command: "go run ./tools/depsage -days", limit: "-days without a value"},
		{name: "not a number", command: "go run ./tools/depsage -days seven", limit: "-days seven, which is not a number"},
		{name: "unknown flag", command: "go run ./tools/depsage -verbose", limit: "-verbose, which is not an argument"},
		{name: "positional", command: "go run ./tools/depsage ./...", limit: "./..., which is not an argument"},
		{name: "computed environment", command: "GOTOOLCHAIN=$(TOOLCHAIN) go run ./tools/depsage", limit: "the computed environment GOTOOLCHAIN=$(TOOLCHAIN)"},
		{name: "computed executable", command: "$(GO) run ./tools/depsage -days 7", limit: "$(GO), an executable"},
		{name: "assignment that is not a variable", command: "1X=2 go run ./tools/depsage", limit: "1X=2, an executable"},
		{name: "quoted value", command: "go run ./tools/depsage -days '7'", limit: "the computed value '7' of -days"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			days, invocation, err := dependencyAgeCommand("Makefile", 3, tc.command)
			if tc.limit != "" {
				if !errors.Is(err, model.ErrOutsideSubset) || !strings.Contains(err.Error(), "Makefile:3: "+tc.limit) {
					t.Fatalf("error = %v, want a limit naming %q", err, tc.limit)
				}
				return
			}
			if err != nil || days != tc.days || invocation != tc.invocation {
				t.Fatalf("command = %d, %v, %v; want %d, %v", days, invocation, err, tc.days, tc.invocation)
			}
		})
	}
}
