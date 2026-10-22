package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// m4CorpusCase is what one payload of an adapter's corpus needs and
// answers: the repository files it reads, written first; what its command
// wrote, written between the pre event derived from it and the payload;
// whether that pre event is sent; the answer; and evidence the reply alone
// cannot show.
type m4CorpusCase struct {
	files, written map[string]string
	pre            bool
	want           m4Want
	check          func(s *suite, repo string) error
}

// m4Adapter is the replay of one adapter: its agent, its row category, the
// corpus directory and the proof manifest under the module root, and what
// each corpus payload needs.
type m4Adapter struct {
	agent, category, corpus, manifest string
	cases                             map[string]m4CorpusCase
}

// m4AdapterRows replays every corpus payload and every proof fixture of
// an adapter through the installed command, one row each.
func (s *suite) m4AdapterRows(ctx context.Context, a m4Adapter) []row {
	return append(s.m4CorpusRows(ctx, a), s.m4FixtureRows(ctx, a)...)
}

// m4CorpusRows replays the corpus, one row per payload file, and a failing
// row when a payload the harness expects is gone.
func (s *suite) m4CorpusRows(ctx context.Context, a m4Adapter) []row {
	entries, err := os.ReadDir(filepath.Join(s.root, filepath.FromSlash(a.corpus)))
	if err != nil {
		return []row{measuredM4(a.category, "corpus", fmt.Errorf("read the corpus: %w", err))}
	}
	var rows []row
	for _, entry := range entries {
		stem, payload := strings.CutSuffix(entry.Name(), ".json")
		if payload && !entry.IsDir() {
			rows = append(rows, measuredM4(a.category, stem, s.m4Corpus(ctx, a, stem)))
		}
	}
	if len(rows) != len(a.cases) {
		rows = append(rows, measuredM4(a.category, "corpus", fmt.Errorf(
			"the corpus holds %d payloads, the harness expects %d: %w", len(rows), len(a.cases), ErrAcceptance)))
	}
	return rows
}

// m4Corpus replays one corpus payload in a repository of its own.
func (s *suite) m4Corpus(ctx context.Context, a m4Adapter, stem string) error {
	c, known := a.cases[stem]
	if !known {
		return fmt.Errorf("the corpus payload %s has no expectation: %w", stem, ErrAcceptance)
	}
	body, err := readFile(filepath.Join(s.root, filepath.FromSlash(a.corpus), stem+".json"))
	if err != nil {
		return err
	}
	after := maps.Clone(c.files)
	if after == nil {
		after = map[string]string{}
	}
	maps.Copy(after, c.written)
	fixture := m4Fixture{payload: body, before: m4Bytes(c.files), after: m4Bytes(after)}
	if c.pre {
		if fixture.pre, err = m4DerivedPre(a.agent, body); err != nil {
			return err
		}
	}
	name := a.category + "-" + stem
	event, replies, err := s.m4Replay(ctx, a.agent, name, fixture)
	if err != nil {
		return err
	}
	if judgeErr := m4Judged(a.agent, event, c.want, replies); judgeErr != nil || c.check == nil {
		return judgeErr
	}
	return c.check(s, filepath.Join(s.base, dirRepos, name))
}

// m4Bytes is a file set as the replay writes it.
func m4Bytes(files map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(files))
	for rel, body := range files {
		out[rel] = []byte(body)
	}
	return out
}

// m4DerivedPre is the pre event of the call a post payload reports: the
// same identities, the host's pre event and none of the fields only a post
// event carries (survey sections 2.5 and 8.4).
func m4DerivedPre(agent string, post []byte) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(post, &payload); err != nil {
		return nil, fmt.Errorf("decode the post payload: %w", err)
	}
	event, drop := `"`+eventPre+`"`, []string{"tool_response"}
	if agent == agentCursor {
		event, drop = `"`+cursorPreTool+`"`, []string{"tool_output", "duration", "error_message", "failure_type", "is_interrupt"}
	}
	for _, key := range drop {
		delete(payload, key)
	}
	payload["hook_event_name"] = json.RawMessage(event)
	pre, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode the derived pre event: %w", err)
	}
	return pre, nil
}

// m4FixtureRows replays the red and the near-miss fixture of every route
// of the adapter's proof manifest, one row each.
func (s *suite) m4FixtureRows(ctx context.Context, a m4Adapter) []row {
	manifest := filepath.Join(s.root, filepath.FromSlash(a.manifest))
	routes, err := m4ReadManifest(manifest)
	if err != nil {
		return []row{measuredM4(a.category, "manifest", err)}
	}
	rows := make([]row, 0, 2*len(routes))
	for _, route := range routes {
		for _, side := range []struct {
			suffix, dir string
			red         bool
		}{{"red", route.Red, true}, {"near", route.Near, false}} {
			name := route.Code + "-" + side.suffix
			dir := filepath.Join(filepath.Dir(manifest), filepath.FromSlash(side.dir))
			rows = append(rows, measuredM4(a.category, name, s.m4FixtureRow(ctx, a, name, dir, side.red)))
		}
	}
	return rows
}

// m4FixtureRow replays one fixture and holds its answer to the route.
func (s *suite) m4FixtureRow(ctx context.Context, a m4Adapter, name, dir string, red bool) error {
	fixture, err := m4LoadFixture(dir)
	if err != nil {
		return err
	}
	event, replies, err := s.m4Replay(ctx, a.agent, a.category+"-"+name, fixture)
	if err != nil {
		return err
	}
	return m4Judged(a.agent, event, m4RouteWant(a.agent, event, red), replies)
}

// m4RouteWant is what a route fixture answers: the near miss is let
// through and the red fixture is refused before the tool or reported after
// it, because a route yields its code when the hook does not allow (D18).
func m4RouteWant(agent, event string, red bool) m4Want {
	switch {
	case !red:
		return m4Allow
	case agent == agentCodex && event == eventPost, agent == agentCursor && cursorKind(event) == cursorPost:
		return m4Block
	default:
		return m4Deny
	}
}
