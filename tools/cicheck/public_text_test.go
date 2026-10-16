package cicheck_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

const privateNameDigest = "88f01e7dc6141404e93d53dcc5cd042e4801a58d6518bad55593d55ff1f25e5f"

// neutralName stands for the private name in the proof fixtures, because a
// fixture must spell the name its rule refuses and the public tree may only
// spell this placeholder.
const neutralName = "acme"

func documentation(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".mdx", ".rst", ".adoc", ".txt":
		return true
	default:
		return false
	}
}

// nameDigest is the only form in which a refused name enters the tree, so
// the private name is never spelled in it.
func nameDigest(name string) string {
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

func newPublicTextInspector(digests []string) func(string, []byte) string {
	forbidden := make(map[string]bool, len(digests))
	for _, digest := range digests {
		forbidden[digest] = true
	}
	return func(path string, data []byte) string {
		text := string(data)
		if documentation(path) && strings.ContainsAny(text, "\u2013\u2014") {
			return "Documentation contains prohibited punctuation."
		}
		tokens := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
			return !unicode.IsLetter(r)
		})
		for _, token := range tokens {
			if forbidden[nameDigest(token)] {
				return "The file names a private organization."
			}
		}
		return ""
	}
}

func inspectPublicText(path string, data []byte) string {
	return publicTextRule().inspect(path, data)
}

func publicTextRule() auditRule {
	return publicTextRuleFor(privateNameDigest)
}

// publicTextRuleFor reads every tracked file for the name, because a name in
// a Go comment or a JSON value is as public as one in a document; the dash
// stays a documentation check here, since text.dashes reads the source.
func publicTextRuleFor(digest string) auditRule {
	return auditRule{
		id: "docs.public-text", rule: "rules/3@1",
		match:   func(string) bool { return true },
		inspect: newPublicTextInspector([]string{digest}),
	}
}

// traceWord is the bare pipeline word of the section 3 boundary, assembled
// from halves so this file carries none. The core trace rule leaves it to
// adopters as ordinary prose, so the kit's own tree is held to it here.
var traceWord = regexp.MustCompile(`(?i)\bwork` + `flow\b`)

func TestKitCarriesNoPipelineTraceWord(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]bool{
		"the work" + "flow ran": true, "WORK" + "FLOW": true,
		".github/work" + "flows/ci.yml": false, "work" + "flow_dispatch": false, "safeWork" + "flow": false,
	} {
		if traceWord.MatchString(text) != want {
			t.Errorf("%q: match %v", text, !want)
		}
	}
	command := exec.CommandContext(t.Context(), "git", "ls-files", "-z")
	command.Dir = filepath.Join("..", "..")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for path := range strings.SplitSeq(string(output), "\x00") {
		if path != "" && traceWord.Match(kitBytes(t, path)) {
			t.Errorf("%s names the pipeline trace word", path)
		}
	}
}

// homePath is an absolute home path of Linux or macOS with a user name,
// assembled from halves so this file carries none. Section 3 keeps every
// such path out of the kit's tree, test inputs included; the core trace
// rule refuses the macOS form only, as adopters write Linux paths of their
// images and runners.
var homePath = regexp.MustCompile(`(?:/ho` + `me|/Us` + `ers)/[A-Za-z0-9_.-]`)

func TestKitCarriesNoHomePath(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]bool{
		"/ho" + "me/u/.config": true, "/Us" + "ers/me": true, "/h/u/.config": false, "home/u": false,
	} {
		if homePath.MatchString(text) != want {
			t.Errorf("%q: match %v", text, !want)
		}
	}
	command := exec.CommandContext(t.Context(), "git", "ls-files", "-z")
	command.Dir = filepath.Join("..", "..")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for path := range strings.SplitSeq(string(output), "\x00") {
		if path != "" && homePath.Match(kitBytes(t, path)) {
			t.Errorf("%s carries an absolute home path", path)
		}
	}
}

// privateTool matches a whole word and the two former file names, because
// fragments inside ordinary words do not name the tool. Its spelling is
// assembled from halves so this file carries none. A go.sum checksum is
// replaced by a blank before the match, because its base64 text holds the
// slash, the plus and the equals sign, word boundaries that would let the
// three letters between two of them read as a word, and a blank never
// joins the text around it into one.
var privateTool = regexp.MustCompile(`(?i)\bp` + `cc(?:_gate\.py|-standards\.json)?\b`)

var privateToolChecksum = regexp.MustCompile(`h1:[A-Za-z0-9+/]+={0,2}`)

func namesPrivateTool(data []byte) bool {
	return privateTool.Match(privateToolChecksum.ReplaceAll(data, []byte(" ")))
}

func TestKitNamesNoPrivateTool(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]bool{
		"the p" + "cc gate": true, "P" + "CC": true, "p" + "Cc": true,
		"(p" + "cc)": true, "tools/p" + "cc/x.py": true,
		"p" + "cc_gate.py": true, "P" + "CC-standards.json": true, "standards_gate.py": false, "standards-gate.json": false,
		"P" + "CC_GATE.PY": true, "p" + "Cc-Standards.Json": true,
		"p" + "ccard": false, "prefixp" + "cc": false,
		"key_p" + "cc": false, "p" + "cc_key": false,
		"7p" + "cc": false, "p" + "cc7": false,
		"example.org/mod v1.0.0 h1:AAp" + "cc" + strings.Repeat("A", 38) + "=\n": false,
		"example.org/mod v1.0.0 h1:/p" + "cc+" + strings.Repeat("A", 38) + "=\n": false,
		"example.org/mod v1.0.0 h1:p" + "cc/" + strings.Repeat("A", 39) + "=\n":  false,
		"h1:/p" + "cc+" + strings.Repeat("A", 38) + "= the p" + "cc gate":        true,
	} {
		if namesPrivateTool([]byte(text)) != want {
			t.Errorf("%q: match %v", text, !want)
		}
	}
	for _, path := range trackedPaths(t) {
		if namesPrivateTool(kitBytes(t, path)) {
			t.Errorf("%s names the private tool", path)
		}
	}
}

// trackedPaths lists the files git tracks in the kit's tree.
func trackedPaths(t *testing.T) []string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", "ls-files", "-z")
	command.Dir = filepath.Join("..", "..")
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for path := range strings.SplitSeq(string(output), "\x00") {
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
