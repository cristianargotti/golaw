package main

import (
	"strings"
	"testing"
)

func TestM7MapTreeIsTheSizeOfTheHub(t *testing.T) {
	t.Parallel()
	tree, holes, ports := m7MapTree()
	if len(holes) != m7Areas*m7HolesPerArea || len(ports) != m7Areas*m7PortsPerArea {
		t.Fatalf("%d holes, %d ports", len(holes), len(ports))
	}
	markers := 0
	for _, body := range tree {
		markers += strings.Count(string(body), "// golaw:hole ")
	}
	if markers != len(holes) {
		t.Fatalf("%d markers for %d holes", markers, len(holes))
	}
}

func TestM7JudgeMapWantsTheBudgetAndTheOrder(t *testing.T) {
	t.Parallel()
	holes, ports := []string{"area0.step-0", "area1.step-0"}, []string{"area0.Port0", "area1.Port0"}
	good := "layers\nport internal/application/{area}\n" +
		"hole internal/application/area0/holes.go:7 area0.step-0 return the answer\n" +
		"hole internal/application/area1/holes.go:7 area1.step-0 return the answer\n" +
		"ports\narea0.Port0 Read0 Read1\narea1.Port0 Read0 Read1\n"
	rows := []struct {
		name, out string
		ok        bool
	}{
		{"near holes first", good, true},
		{"red a port before a hole", strings.Replace(good, "layers\n", "layers\narea0.Port0\n", 1), false},
		{"red a hole missing", strings.Replace(good, "area1.step-0", "area1.step-9", 1), false},
		{"red over the budget", good + strings.Repeat("x", 4*2001), false},
	}
	for _, row := range rows {
		if err := m7JudgeMap(row.out, holes, ports); (err == nil) != row.ok {
			t.Fatalf("%s: %v", row.name, err)
		}
	}
	if m7Tokens(strings.Repeat("a", 4000)) != 1000 || m7Tokens("abc") != 1 {
		t.Fatal("tokens are bytes over four, rounded up")
	}
}
