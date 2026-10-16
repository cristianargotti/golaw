package cicheck_test

import (
	"slices"

	"gopkg.in/yaml.v3"
)

func scalarList(node *yaml.Node) []string {
	nodes := children(node)
	result := make([]string, 0, len(nodes))
	for _, child := range nodes {
		result = append(result, child.Value)
	}
	return result
}

func inspectArtifacts(_ string, data []byte) string {
	root, err := document(data)
	if err != nil {
		return "Release configuration cannot be decoded."
	}
	required := []struct {
		path []string
		want string
	}{
		{[]string{"version"}, "2"},
		{[]string{"project_name"}, "golaw"},
		{[]string{"builds", "0", "main"}, "./cmd/golaw"},
		{[]string{"builds", "0", "id"}, "golaw"},
		{[]string{"archives", "0", "id"}, "binaries"},
		{[]string{"checksum", "name_template"}, "checksums.txt"},
		{[]string{"checksum", "algorithm"}, "sha256"},
		{[]string{"sboms", "0", "artifacts"}, "binary"},
		{[]string{"sboms", "0", "cmd"}, "syft"},
		{[]string{"signs", "0", "cmd"}, "cosign"},
		{[]string{"signs", "0", "artifacts"}, "checksum"},
		{[]string{"signs", "0", "signature"}, "${artifact}.sigstore.json"},
		{[]string{"release", "draft"}, "true"},
		{[]string{"release", "target_commitish"}, "{{ .Commit }}"},
	}
	for _, field := range required {
		if value(at(root, field.path...)) != field.want {
			return "Required release artifact configuration is missing."
		}
	}
	return inspectArtifactLists(root)
}

func inspectArtifactLists(root *yaml.Node) string {
	required := []struct {
		path []string
		want []string
	}{
		{[]string{"builds", "0", "goos"}, []string{"darwin", "linux", "windows"}},
		{[]string{"builds", "0", "goarch"}, []string{"amd64", "arm64"}},
		{[]string{"builds", "0", "env"}, []string{"CGO_ENABLED=0"}},
		{[]string{"builds", "0", "flags"}, []string{"-trimpath"}},
		{[]string{"archives", "0", "ids"}, []string{"golaw"}},
		{[]string{"archives", "0", "formats"}, []string{"binary"}},
		{[]string{"sboms", "0", "ids"}, []string{"binaries"}},
		{[]string{"signs", "0", "args"}, []string{
			"sign-blob", "--new-bundle-format", "--bundle=${signature}", "--yes", "${artifact}",
		}},
	}
	for _, field := range required {
		if !slices.Equal(scalarList(at(root, field.path...)), field.want) {
			return "Release artifact coverage or signing arguments differ."
		}
	}
	if len(children(at(root, "builds"))) != 1 {
		return "Release must have exactly one six-target build."
	}
	return ""
}

func artifactRule() auditRule {
	return auditRule{
		id: "release.assets", rule: "rules/4.2@1",
		match:   func(path string) bool { return path == ".goreleaser.yml" },
		inspect: inspectArtifacts,
	}
}
