package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cristianargotti/golaw/internal/genmap"
)

// The map rows of M7.13 row 8 and the size of their fixture: the 16
// adapters of the hub, 6 areas of 2 ports with 4 methods each and 30 open
// holes, more than a slice of every kind opens, so the budget and the
// order are measured where the map must cut.
const (
	m7MapCategory    = "m7-map"
	m7Areas          = 6
	m7PortsPerArea   = 2
	m7MethodsPerPort = 4
	m7HolesPerArea   = 5
)

// m7Ports is the ports.go of one area of the map fixture.
func m7Ports(area string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "// Package %s holds the ports of one area of the fixture.\npackage %s\n\nimport \"context\"\n", area, area)
	for p := range m7PortsPerArea {
		fmt.Fprintf(&b, "\n// Port%d reads the values of the area.\ntype Port%d interface {\n", p, p)
		for m := range m7MethodsPerPort {
			fmt.Fprintf(&b, "\t// Read%d reads one value by id.\n\tRead%d(ctx context.Context, id string) (string, error)\n", m, m)
		}
		fmt.Fprintf(&b, "}\n")
	}
	return []byte(b.String())
}

// m7Holes is the holes.go of one area and the ids of its open holes.
func m7Holes(area string) ([]byte, []string) {
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n", area)
	ids := make([]string, 0, m7HolesPerArea)
	for i := range m7HolesPerArea {
		id := fmt.Sprintf("%s.step-%d", area, i)
		fmt.Fprintf(&b, "\nfunc step%d() error {\n\t// golaw:hole %s return the answer of step %d\n\treturn errNotReady\n}\n", i, id, i)
		ids = append(ids, id)
	}
	return []byte(b.String()), ids
}

// m7MapTree is a repository the size of the hub: it answers the tree, the
// ids of its open holes and its port names, each in order.
func m7MapTree() (map[string][]byte, []string, []string) {
	tree := map[string][]byte{"go.mod": []byte(m7HubModule)}
	for path, body := range m7HubTree(false) {
		if strings.HasPrefix(path, "internal/adapters/in/") || strings.HasPrefix(path, "internal/adapters/out/") {
			tree[path] = body
		}
	}
	holeIDs := make([]string, 0, m7Areas*m7HolesPerArea)
	ports := make([]string, 0, m7Areas*m7PortsPerArea)
	for a := range m7Areas {
		area := fmt.Sprintf("area%d", a)
		dir := "internal/application/" + area + "/"
		holesFile, ids := m7Holes(area)
		tree[dir+"ports.go"], tree[dir+"holes.go"] = m7Ports(area), holesFile
		tree[dir+"errors.go"] = []byte("package " + area + "\n\nimport \"errors\"\n\nvar errNotReady = errors.New(\"not ready\")\n")
		holeIDs = append(holeIDs, ids...)
		for p := range m7PortsPerArea {
			ports = append(ports, fmt.Sprintf("%s.Port%d", area, p))
		}
	}
	return tree, holeIDs, ports
}

// m7BytesPerToken is the estimate of A52: a token is four bytes.
const m7BytesPerToken = 4

// m7Tokens estimates the tokens of text as A52 does: bytes over four,
// rounded up.
func m7Tokens(text string) int {
	return (len(text) + m7BytesPerToken - 1) / m7BytesPerToken
}

// m7JudgeMap requires the map within its budget, every open hole listed
// and every hole before the first port the map prints (M7.8); a port cut
// by the budget is not required.
func m7JudgeMap(out string, holeIDs, ports []string) error {
	if tokens := m7Tokens(out); tokens > genmap.DefaultBudget {
		return fmt.Errorf("the map takes %d tokens, over the budget of %d: %w", tokens, genmap.DefaultBudget, ErrAcceptance)
	}
	last := -1
	for _, id := range holeIDs {
		at := strings.LastIndex(out, id)
		if at < 0 {
			return fmt.Errorf("the map does not list the open hole %s: %w", id, ErrAcceptance)
		}
		last = max(last, at)
	}
	for _, port := range ports {
		if at := strings.Index(out, port); at >= 0 && at < last {
			return fmt.Errorf("the map prints the port %s before an open hole: %w", port, ErrAcceptance)
		}
	}
	return nil
}

// m7MapCases are the rows of M7.13 row 8.
func m7MapCases() []m7Case {
	return []m7Case{
		{name: "budget", run: func(s *m7Suite, ctx context.Context) error {
			out, _, _, err := s.m7Map(ctx, "map-budget")
			if err != nil {
				return err
			}
			return m7JudgeMap(out, nil, nil)
		}},
		{name: "holes-first", run: func(s *m7Suite, ctx context.Context) error {
			out, holeIDs, ports, err := s.m7Map(ctx, "map-holes-first")
			if err != nil {
				return err
			}
			return m7JudgeMap(out, holeIDs, ports)
		}},
	}
}

// m7Map adopts the map fixture, commits it and answers what golaw map
// printed with the fixture's hole ids and port names.
func (s *m7Suite) m7Map(ctx context.Context, name string) (string, []string, []string, error) {
	tree, holeIDs, ports := m7MapTree()
	dir, got, err := s.m7Adopted(ctx, name, "go", tree)
	if err != nil {
		return "", nil, nil, err
	}
	if err := success(got); err != nil {
		return "", nil, nil, fmt.Errorf("init over the map fixture: %w", err)
	}
	if err := s.m6Commit(ctx, dir, m7BaseMessage); err != nil {
		return "", nil, nil, err
	}
	printed := s.m5Golaw(ctx, dir, "map")
	if err := success(printed); err != nil {
		return "", nil, nil, fmt.Errorf("golaw map: %w", err)
	}
	return printed.stdout, holeIDs, ports, nil
}
