//go:build network

package cicheck_test

import (
	"maps"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestActionTagsResolveToPins(t *testing.T) {
	t.Parallel()
	tree := kitTree(t)
	seen := make(map[string]string)
	for _, file := range tree.Files() {
		if !ciPath(file.Path) {
			continue
		}
		data, err := tree.Read(file.Path)
		if err != nil {
			t.Fatal(err)
		}
		verifyActionPins(t, data, seen)
	}
	documents := m3PinDocuments(t)
	for _, name := range slices.Sorted(maps.Keys(documents)) {
		verifyActionPins(t, documents[name], seen)
	}
	if len(seen) == 0 {
		t.Fatal("no action pins measured")
	}
}

func verifyActionPins(t *testing.T, data []byte, seen map[string]string) {
	t.Helper()
	root, err := document(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range named(root, "uses") {
		verifyActionPin(t, node, seen)
	}
}

func verifyActionPin(t *testing.T, node *yaml.Node, seen map[string]string) {
	t.Helper()
	action, sha, found := strings.Cut(node.Value, "@")
	if !found {
		t.Fatalf("unpinned action: %s", node.Value)
	}
	parts := strings.Split(action, "/")
	if len(parts) < 2 {
		t.Fatalf("invalid action: %s", action)
	}
	version := strings.TrimSpace(strings.TrimPrefix(node.LineComment, "#"))
	endpoint := "repos/" + strings.Join(parts[:2], "/") + "/commits/" + version
	if previous, ok := seen[endpoint]; ok {
		if previous != sha {
			t.Fatal("one version names different pins")
		}
		return
	}
	command := exec.CommandContext(t.Context(), "gh", "api", endpoint, "--jq", ".sha")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("resolve %s: %v", endpoint, err)
	}
	if strings.TrimSpace(string(output)) != sha {
		t.Fatalf("tag does not resolve to %s: %s", sha, endpoint)
	}
	seen[endpoint] = sha
}
