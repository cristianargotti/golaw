package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// The route proof convention of the adapters (D18): a fixture's payload,
// its optional pre event, the repository at that pre event under before/,
// the suffix of a file that spells a prohibited rune, and the root every
// payload of the module names.
const (
	m4PayloadFile = "payload.json"
	m4PreFile     = "pre.json"
	m4BeforeDir   = "before/"
	m4EscSuffix   = ".esc"
	m4ProjectRoot = "/project"
)

// escapes are the sequences a .esc file spells: \\ and \uXXXX.
var escapes = regexp.MustCompile(`\\\\|\\u[0-9a-fA-F]{4}`)

// m4Route is one entry of an adapter's proof manifest.
type m4Route struct {
	ID      string `yaml:"id"`
	Code    string `yaml:"code"`
	Red     string `yaml:"red"`
	Near    string `yaml:"near"`
	Profile string `yaml:"profile"`
}

// m4Fixture is one fixture directory read as hooktest.Prove reads it.
type m4Fixture struct {
	payload, pre  []byte
	before, after map[string][]byte
}

// m4ReadManifest reads an adapter's proof manifest strictly, every entry
// naming its id, code and two fixtures.
func m4ReadManifest(path string) ([]m4Route, error) {
	body, err := readFile(path)
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Checks []m4Route `yaml:"checks"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf(decodeFormat, path, err)
	}
	if len(manifest.Checks) == 0 {
		return nil, fmt.Errorf("%s lists no route: %w", path, ErrAcceptance)
	}
	for _, route := range manifest.Checks {
		if route.ID == "" || route.Code == "" || route.Red == "" || route.Near == "" {
			return nil, fmt.Errorf("%s holds an incomplete route %+v: %w", path, route, ErrAcceptance)
		}
	}
	return manifest.Checks, nil
}

// m4LoadFixture reads a fixture directory, a .esc file decoded and its
// suffix dropped.
func m4LoadFixture(dir string) (m4Fixture, error) {
	fixture := m4Fixture{before: map[string][]byte{}, after: map[string][]byte{}}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		body, err := readFile(path)
		if err != nil {
			return err
		}
		fixture.add(filepath.ToSlash(rel), body)
		return nil
	})
	if err != nil {
		return m4Fixture{}, fmt.Errorf("read the fixture %s: %w", dir, err)
	}
	if fixture.payload == nil {
		return m4Fixture{}, fmt.Errorf("the fixture %s holds no %s: %w", dir, m4PayloadFile, ErrAcceptance)
	}
	return fixture, nil
}

func (f *m4Fixture) add(rel string, body []byte) {
	if plain, escaped := strings.CutSuffix(rel, m4EscSuffix); escaped {
		rel, body = plain, m4Unescape(body)
	}
	switch {
	case rel == m4PayloadFile:
		f.payload = body
	case rel == m4PreFile:
		f.pre = body
	case strings.HasPrefix(rel, m4BeforeDir):
		f.before[strings.TrimPrefix(rel, m4BeforeDir)] = body
	default:
		f.after[rel] = body
	}
}

// m4Unescape decodes the \uXXXX and \\ sequences of a .esc file and keeps
// every other byte, so a fixture spells a prohibited rune without holding
// it.
func m4Unescape(text []byte) []byte {
	return escapes.ReplaceAllFunc(text, func(sequence []byte) []byte {
		decoded, err := strconv.Unquote(`"` + string(sequence) + `"`)
		if err != nil {
			return sequence
		}
		return []byte(decoded)
	})
}

// m4Payload names the repository wherever a payload names the root of the
// module's fixtures, in slashes, as hooktest.Payload does.
func m4Payload(data []byte, repo string) []byte {
	return bytes.ReplaceAll(data, []byte(m4ProjectRoot), []byte(filepath.ToSlash(repo)))
}

// m4Events are the events an agent's hooks file holds handlers for.
func m4Events(agent string) []string {
	if agent == agentCodex {
		return []string{eventPre, eventPost}
	}
	return []string{cursorPreTool, cursorReadFile, cursorShellExec, cursorPostTool, cursorPostFailure}
}

// m4RouteOf is the installed handler a payload reaches: its own event and
// tool when they are the agent's, the shell pre event otherwise, whose
// command is the same golaw command, so a payload of another agent or one
// that does not decode still reaches golaw, as the envelope routes need.
func m4RouteOf(agent string, payload []byte) (string, string) {
	var head struct {
		Event string `json:"hook_event_name"`
		Tool  string `json:"tool_name"`
	}
	if json.Unmarshal(payload, &head) == nil && slices.Contains(m4Events(agent), head.Event) {
		return head.Event, head.Tool
	}
	if agent == agentCodex {
		return eventPre, toolBash
	}
	return cursorShellExec, ""
}

// m4Materialize writes the files into the repository and removes every
// file of previous the files no longer hold.
func m4Materialize(repo string, files, previous map[string][]byte) error {
	for rel := range previous {
		if _, kept := files[rel]; kept {
			continue
		}
		if err := os.Remove(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
			return fmt.Errorf("remove %s: %w", rel, err)
		}
	}
	for rel, body := range files {
		if err := writeFile(filepath.Join(repo, filepath.FromSlash(rel)), body, fileMode); err != nil {
			return err
		}
	}
	return nil
}

// m4Replay runs a fixture the way hooktest.Prove judges it, through the
// installed command: the repository at the pre event, the pre event, which
// must be allowed so its baseline is kept, then the repository the payload
// is judged over and the payload. It answers the event the payload was
// routed by and its replies.
func (s *suite) m4Replay(ctx context.Context, agent, name string, f m4Fixture) (string, []m4Reply, error) {
	repo, err := s.repository(ctx, name, true)
	if err != nil {
		return "", nil, err
	}
	t := s.m4Shared(repo)
	if beforeErr := m4Materialize(repo, f.before, nil); beforeErr != nil {
		return "", nil, beforeErr
	}
	if f.pre != nil {
		if preErr := s.m4Allowed(ctx, t, agent, m4Payload(f.pre, repo)); preErr != nil {
			return "", nil, fmt.Errorf("the pre event of %s: %w", name, preErr)
		}
	}
	if afterErr := m4Materialize(repo, f.after, f.before); afterErr != nil {
		return "", nil, afterErr
	}
	payload := m4Payload(f.payload, repo)
	event, tool := m4RouteOf(agent, payload)
	replies, err := s.m4Invoke(ctx, t, agent, event, tool, payload)
	return event, replies, err
}

// m4Allowed sends a payload and requires every shell to allow it.
func (s *suite) m4Allowed(ctx context.Context, t m4Target, agent string, payload []byte) error {
	event, tool := m4RouteOf(agent, payload)
	replies, err := s.m4Invoke(ctx, t, agent, event, tool, payload)
	if err != nil {
		return err
	}
	return m4Judged(agent, event, m4Allow, replies)
}

// m4Judged holds every reply to the answer the row wants, naming the shell
// whose reply failed.
func m4Judged(agent, event string, want m4Want, replies []m4Reply) error {
	judge := judgeCodex
	if agent == agentCursor {
		judge = judgeCursor
	}
	for _, reply := range replies {
		if _, err := judge(event, want, reply.got); err != nil {
			return fmt.Errorf("%s through %s: %w", event, reply.shell, err)
		}
	}
	return nil
}
