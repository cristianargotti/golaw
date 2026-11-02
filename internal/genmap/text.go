package genmap

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/holes"
)

// bytesPerToken estimates a token as four bytes (A52).
const bytesPerToken = 4

// moreReserve is the room kept for the five count lines, so the text stays
// within the budget whatever the sections omit.
const moreReserve = 400

// section is one part of the text: the plural its count line names and
// its lines with the name each one carries.
type section struct {
	plural       string
	names, lines []string
}

// Text prints the map within budget tokens: the layout, every open hole,
// then ports, use cases, adapters, handlers and contracts, each section cut
// where the next line would pass the budget and closed by a count of what
// it left out with the command that prints it (M7.8).
func (r Report) Text(budget int) string {
	limit := budget*bytesPerToken - moreReserve
	lines := []string{"golaw map layout: " + strings.Join(r.Layers, ", ")}
	for _, h := range r.Holes {
		lines = append(lines, strings.TrimRight(fmt.Sprintf("hole open %s:%d %s %s", h.Path, h.Line, h.ID, h.Constraint), " "))
	}
	size := 0
	for _, line := range lines {
		size += len(line) + 1
	}
	for _, s := range r.sections() {
		for i, line := range s.lines {
			if size+len(line)+1 > limit {
				lines = append(lines, "and "+strconv.Itoa(len(s.lines)-i)+" more "+s.plural+": "+moreCommand(s.names[i:]))
				break
			}
			lines, size = append(lines, line), size+len(line)+1
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// sections lays out the item sections in their truncation order.
func (r Report) sections() []section {
	contractLines := make([]string, 0, len(r.Contracts))
	for _, c := range r.Contracts {
		contractLines = append(contractLines, "contract "+c)
	}
	return []section{
		itemSection(wordPort, "ports", r.Ports),
		itemSection("usecase", "use cases", r.Usecases),
		itemSection("adapter", "adapters", r.Adapters),
		itemSection("handler", "handlers", r.Handlers),
		{plural: "contracts", names: make([]string, len(r.Contracts)), lines: contractLines},
		handwrittenSection(r.Handwritten),
	}
}

// handwrittenSection writes each marked file as "handwritten <path>", its
// reason after a colon.
func handwrittenSection(items []Item) section {
	s := section{plural: "handwritten files", names: make([]string, len(items))}
	for _, it := range items {
		line := "handwritten " + it.Path
		if it.Note != "" {
			line += ": " + it.Note
		}
		s.lines = append(s.lines, line)
	}
	return s
}

// itemSection writes each item as "<word> <name> <path>", its details
// after a colon and its note in parentheses.
func itemSection(word, plural string, items []Item) section {
	s := section{plural: plural}
	for _, it := range items {
		line := word + " " + it.Name + " " + it.Path
		if len(it.Details) > 0 {
			line += ": " + strings.Join(it.Details, "; ")
		}
		if it.Note != "" {
			line += " (" + it.Note + ")"
		}
		s.names, s.lines = append(s.names, it.Name), append(s.lines, line)
	}
	return s
}

// moreCommand names the command that prints what a section left out: the
// map of the one area every omitted item shares, else the whole report.
func moreCommand(names []string) string {
	area := ""
	for i, name := range names {
		a, _, ok := strings.Cut(name, ".")
		if !ok || a == "" || i > 0 && a != area {
			return "golaw map --json"
		}
		area = a
	}
	if area == "" {
		return "golaw map --json"
	}
	return "golaw map --area " + area
}

// The wire form of the report: lower case keys and every section an array,
// so a reader never meets null.
type (
	wireReport struct {
		Layers    []string   `json:"layers"`
		Holes     []wireHole `json:"holes"`
		Ports     []wireItem `json:"ports"`
		Usecases  []wireItem `json:"usecases"`
		Adapters  []wireItem `json:"adapters"`
		Handlers  []wireItem `json:"handlers"`
		Contracts []string   `json:"contracts"`
		// Handwritten is every marked file no other section holds.
		Handwritten []wireItem `json:"handwritten"`
	}
	wireHole struct {
		ID         string `json:"id"`
		Constraint string `json:"constraint"`
		Path       string `json:"path"`
		Line       int    `json:"line"`
		Func       string `json:"func"`
	}
	wireItem struct {
		Name    string   `json:"name"`
		Path    string   `json:"path"`
		Details []string `json:"details"`
		Note    string   `json:"note,omitempty"`
	}
)

// JSON marshals the whole report, indented by two spaces.
func (r Report) JSON() ([]byte, error) {
	w := wireReport{
		Layers: nonNil(r.Layers), Ports: wireItems(r.Ports), Usecases: wireItems(r.Usecases),
		Adapters: wireItems(r.Adapters), Handlers: wireItems(r.Handlers), Contracts: nonNil(r.Contracts),
		Holes: wireHoles(r.Holes), Handwritten: wireItems(r.Handwritten),
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal the map: %w", err)
	}
	return data, nil
}

func wireHoles(hs []holes.Hole) []wireHole {
	out := make([]wireHole, 0, len(hs))
	for _, h := range hs {
		out = append(out, wireHole{ID: h.ID, Constraint: h.Constraint, Path: h.Path, Line: h.Line, Func: h.Func})
	}
	return out
}

func wireItems(items []Item) []wireItem {
	out := make([]wireItem, 0, len(items))
	for _, it := range items {
		out = append(out, wireItem{Name: it.Name, Path: it.Path, Details: nonNil(it.Details), Note: it.Note})
	}
	return out
}

// nonNil answers an empty slice for nil, which JSON writes as [].
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
