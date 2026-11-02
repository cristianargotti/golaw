package usecase

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/holes"
)

// words splits a Go name into lower case words the way it reads, keeping an
// initialism together; it splits the kebab spelling of a hole id, so the
// action text and the hole id of one use case never disagree on a word.
func words(name string) []string {
	return strings.Split(holes.Kebab(name), "-")
}

// snake is the file name stem of a use case: "ReadPlan" is "read_plan".
func snake(name string) string {
	return strings.Join(words(name), "_")
}

// action is the prefix of every error a use case answers.
func action(area, name string) string {
	return area + ": " + strings.Join(words(name), " ")
}

// portList names the ports as a sentence does: "Ledger, Mailer and Clock".
func portList(ports []string) string {
	if len(ports) < 2 {
		return strings.Join(ports, "")
	}
	last := len(ports) - 1
	return strings.Join(ports[:last], ", ") + " and " + ports[last]
}
