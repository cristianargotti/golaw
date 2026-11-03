package genmap

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ctxSection is one section of the context text: its title and the lines
// of each entry, the first line the name and position, the others indented.
type ctxSection struct {
	title   string
	entries [][]string
}

// sections lays out the context in its order of value; the budget drops
// entries from the last section first.
func (c Context) sections() []ctxSection {
	commands := ctxSection{title: "commands"}
	for _, cmd := range c.Commands {
		commands.entries = append(commands.entries, []string{cmd})
	}
	return []ctxSection{
		declSection(wordPort+"s", c.Ports),
		declSection("use cases", c.Usecases),
		declSection("types", c.Types),
		declSection("adapters", c.Adapters),
		declSection("handlers", c.Handlers),
		declSection("tables", c.Tables),
		declSection("queries", c.Queries),
		declSection("wiring", c.Wiring),
		declSection("patterns", c.Patterns),
		commands,
	}
}

// declSection writes each declaration as "<name> <at>" and its lines.
func declSection(title string, decls []Decl) ctxSection {
	s := ctxSection{title: title}
	for _, d := range decls {
		entry := make([]string, 0, 1+len(d.Lines))
		entry = append(entry, strings.TrimSpace(d.Name+" "+d.At))
		for _, line := range d.Lines {
			entry = append(entry, "  "+line)
		}
		s.entries = append(s.entries, entry)
	}
	return s
}

// entrySize is the bytes an entry takes, a newline after each line.
func entrySize(lines []string) int {
	n := 0
	for _, line := range lines {
		n += len(line) + 1
	}
	return n
}

// Text prints the context within budget tokens, a token estimated as four
// bytes: a heading, then each section that holds an entry. Over the
// budget, the entries of the last section go first, then the section
// before it, each cut closed by a count with the command that prints all;
// a section cut whole keeps its count line alone.
func (c Context) Text(budget int) string {
	limit := budget * bytesPerToken
	head := "golaw context " + c.Area + ": signatures only, by layer; open only the file you edit"
	secs := c.sections()
	kept := c.kept(secs, limit-len(head)-1)
	lines := []string{head}
	for i, s := range secs {
		if kept[i] > 0 {
			lines = append(lines, "## "+s.title)
		}
		for _, e := range s.entries[:kept[i]] {
			lines = append(lines, e...)
		}
		if dropped := len(s.entries) - kept[i]; dropped > 0 {
			lines = append(lines, c.moreLine(s, dropped))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// kept answers how many entries of each section fit in limit bytes,
// dropping from the last section first and counting the line that closes
// a cut section.
func (c Context) kept(secs []ctxSection, limit int) []int {
	kept := make([]int, len(secs))
	total := 0
	for i, s := range secs {
		kept[i] = len(s.entries)
		if kept[i] > 0 {
			total += len(s.title) + len("## \n")
		}
		for _, e := range s.entries {
			total += entrySize(e)
		}
	}
	for i := len(secs) - 1; i >= 0 && total > limit; i-- {
		if kept[i] > 0 {
			total += len(c.moreLine(secs[i], len(secs[i].entries))) + 1
		}
		for kept[i] > 0 && total > limit {
			kept[i]--
			total -= entrySize(secs[i].entries[kept[i]])
		}
		if kept[i] == 0 && len(secs[i].entries) > 0 {
			total -= len(secs[i].title) + len("## \n")
		}
	}
	return kept
}

// moreLine counts the entries a section left out and names the command
// that prints them.
func (c Context) moreLine(s ctxSection, dropped int) string {
	return "... " + strconv.Itoa(dropped) + " more " + s.title + ": golaw context " + c.Area + " --json"
}

// The wire form of the context: lower case keys and every section an
// array, so a reader never meets null.
type (
	wireContext struct {
		Area     string     `json:"area"`
		Ports    []wireDecl `json:"ports"`
		Usecases []wireDecl `json:"usecases"`
		Types    []wireDecl `json:"types"`
		Adapters []wireDecl `json:"adapters"`
		Handlers []wireDecl `json:"handlers"`
		Tables   []wireDecl `json:"tables"`
		Queries  []wireDecl `json:"queries"`
		Wiring   []wireDecl `json:"wiring"`
		Patterns []wireDecl `json:"patterns"`
		Commands []string   `json:"commands"`
	}
	wireDecl struct {
		Name  string   `json:"name"`
		At    string   `json:"at"`
		Lines []string `json:"lines"`
	}
)

// JSON marshals the whole context, indented by two spaces.
func (c Context) JSON() ([]byte, error) {
	w := wireContext{
		Area: c.Area, Ports: wireDecls(c.Ports), Usecases: wireDecls(c.Usecases), Types: wireDecls(c.Types),
		Adapters: wireDecls(c.Adapters), Handlers: wireDecls(c.Handlers), Tables: wireDecls(c.Tables),
		Queries: wireDecls(c.Queries), Wiring: wireDecls(c.Wiring), Patterns: wireDecls(c.Patterns),
		Commands: nonNil(c.Commands),
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal the context: %w", err)
	}
	return data, nil
}

func wireDecls(decls []Decl) []wireDecl {
	out := make([]wireDecl, 0, len(decls))
	for _, d := range decls {
		out = append(out, wireDecl{Name: d.Name, At: d.At, Lines: nonNil(d.Lines)})
	}
	return out
}
