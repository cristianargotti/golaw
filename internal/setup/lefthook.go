package setup

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

// rungs are the git hooks setup arms, each routed to the golaw hook of
// its name.
func rungs() []string { return []string{"pre-commit", "commit-msg", "pre-push"} }

const routesFix = "apply the lefthook.yml proposal golaw init prints, then run golaw setup"

// requireRoutes refuses a lefthook.yml that does not run golaw hook <rung>
// for every rung: init only proposes over a lefthook.yml the repository
// already holds, so its own commands would run, or lefthook would create
// no hook, while setup recorded the rungs as armed. The local file
// lefthook merges over it counts too. It refuses only what it reads with
// certainty: a configuration that extends another one, or a section that
// names its command through an alias, may route the rung from elsewhere
// and is left to lefthook.
func requireRoutes(files ...[]byte) error {
	config := map[string]string{}
	for _, data := range files {
		for key, body := range sections(data) {
			config[key] += body
		}
	}
	_, extends := config["extends"]
	_, remotes := config["remotes"]
	if extends || remotes {
		return nil
	}
	var missing []string
	for _, rung := range rungs() {
		body := config[rung]
		if !routed(body, rung) && !aliased.MatchString(body) {
			missing = append(missing, "no "+rung+" command golaw hook "+rung)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("%w: lefthook.yml has %s; %s", install.ErrPath, strings.Join(missing, ", "), routesFix)
	}
	return nil
}

// aliased finds a YAML alias, a * that starts a plain value; a quoted
// glob such as "*.go" is no alias.
var aliased = regexp.MustCompile(`(^|[\s\[{,])\*[^\s,\]}]+`)

// sections splits a lefthook.yml into its top-level keys, each with the
// text under it and the rest of its own line, comments left out.
func sections(data []byte) map[string]string {
	config := map[string]string{}
	key := ""
	for raw := range strings.SplitSeq(string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))), "\n") {
		line := uncomment(raw)
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "---") {
			continue
		}
		if name, rest, found := strings.Cut(line, ":"); found && !strings.ContainsAny(line[:1], " \t-") {
			key = strings.Trim(strings.TrimSpace(name), `"'`)
			config[key] += rest + "\n"
			continue
		}
		if key != "" {
			config[key] += line + "\n"
		}
	}
	return config
}

// uncomment drops a YAML comment: a # that starts the line or follows a
// space or a tab.
func uncomment(line string) string {
	for i := range len(line) {
		if line[i] == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
			return line[:i]
		}
	}
	return line
}

// routed reports whether the text of a hook section calls golaw hook
// <rung>, golaw by its name or by a path to it.
func routed(body, rung string) bool {
	fields := strings.Fields(body)
	for i := 0; i+2 < len(fields); i++ {
		name := strings.TrimSuffix(path.Base(strings.ReplaceAll(fields[i], `\`, "/")), ".exe")
		name = strings.TrimLeft(name, `"'{[`)
		if name == "golaw" && fields[i+1] == "hook" && strings.Trim(fields[i+2], `"'{}[],`) == rung {
			return true
		}
	}
	return false
}
