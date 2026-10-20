package bootstrap_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/bootstrap"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

func TestManagedPolicyPreservesReviewedSurroundings(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, before, version, prefix, suffix string
	}{
		{
			"single quotes", "kit: {version: 'old''pin'} # pin\nmanaged: []\nlanguage: en # keep\n", "new'pin",
			"kit: {version: 'new''pin'} # pin\nmanaged:", "\nlanguage: en # keep\n",
		},
		{
			"double quotes", "kit: {version: \"old\\\"pin\"}\nmanaged: [] # list\n", "new\"pin",
			"kit: {version: \"new\\\"pin\"}\nmanaged:", " # list\n",
		},
		{
			"unicode column", "kit: {\"version\": \"versão\"} # ação\nmanaged: []\n", "próxima",
			"kit: {\"version\": \"próxima\"} # ação\nmanaged:", "\n",
		},
		{
			"unicode prefix", "{language: ação, kit: {version: old}, managed: []}\n", "new",
			"{language: ação, kit: {version: new}, managed: [", "\n",
		},
		{
			"block", "# head\nkit:\n  version: old # pin\nmanaged: # assets\n  - path: old\n    sha256: old # tail\n# footer\nlanguage: en\n",
			"new", "# head\nkit:\n  version: new # pin\nmanaged: # assets\n  ", " # tail\n# footer\nlanguage: en\n",
		},
		{
			"head comment", "kit: {version: old}\nmanaged:\n  # assets\n  - path: old\n    sha256: old\n",
			"new", "kit: {version: new}\nmanaged:\n  # assets\n  ", "\n",
		},
		{
			"indentless", "kit: {version: old}\nmanaged:\n- path: old\n  sha256: old\nlanguage: en\n",
			"new", "kit: {version: new}\nmanaged:\n", "\nlanguage: en\n",
		},
		{
			"flow root", "{golaw: 1, kit: {version: old}, managed: []} # end\n", "new",
			"{golaw: 1, kit: {version: new}, managed: [", "]} # end\n",
		},
		{
			"flow contents", "kit: {version: old}\nmanaged: [{path: 'old]', sha256: \"old}\"}]\nlanguage: en\n",
			"new", "kit: {version: new}\nmanaged:", "\nlanguage: en\n",
		},
		{
			"flow hash", "kit: {version: old}\nmanaged: [{path: old#file, sha256: old}]\nlanguage: en\n",
			"new", "kit: {version: new}\nmanaged:", "\nlanguage: en\n",
		},
		{
			"multiline flow", "kit: {version: old}\nmanaged: [\n {path: old, sha256: old}\n]\nlanguage: en\n",
			"new", "kit: {version: new}\nmanaged: [", "\nlanguage: en\n",
		},
		{
			"crlf", "kit: {version: old}\r\nmanaged: []\r\nlanguage: en\r\n", "new",
			"kit: {version: new}\r\nmanaged:", "\r\nlanguage: en\r\n",
		},
		{"missing newline", "kit: {version: old}", "new", "kit: {version: new}\nmanaged:", "\n"},
		{"indented root", "  kit: {version: old}\n", "new", "  kit: {version: new}\n  managed:", "\n"},
		{"null list", "kit: {version: old}\nmanaged: null\n", "new", "kit: {version: new}\nmanaged:", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			managed := []policy.ManagedFile{{Path: "config.txt", SHA256: "new"}}
			got, err := bootstrap.ManagedPolicy([]byte(tc.before), tc.version, managed)
			if err != nil || !bytes.HasPrefix(got, []byte(tc.prefix)) || !bytes.HasSuffix(got, []byte(tc.suffix)) {
				t.Fatalf("splice = %q, %v", got, err)
			}
			decoded, err := policy.Decode(got)
			if err != nil || decoded.Kit.Version != tc.version || !slices.Equal(decoded.Managed, managed) {
				t.Fatalf("decoded = %+v, %v", decoded, err)
			}
			again, err := bootstrap.ManagedPolicy(got, tc.version, managed)
			if err != nil || !bytes.Equal(got, again) {
				t.Fatalf("second splice = %q, %v", again, err)
			}
		})
	}
}

func TestManagedPolicyRefusesUnlocatableValuesBeforeReturningBytes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, reason string
	}{
		{"unknown", "kit: {version: old}\nunknown: true\n", "field unknown"},
		{"duplicate", "kit: {version: old}\nkit: {version: old}\n", "already defined"},
		{"documents", "kit: {version: old}\n---\nkit: {version: old}\n", "more than one document"},
		{"empty", "", "empty"},
		{"null", "null\n", "expected a mapping"},
		{"absent pin", "golaw: 1\n", "kit.version"},
		{"anchor", "kit: {version: &pin old}\n", "anchors"},
		{"tag", "kit: {version: !!str old}\n", "tags"},
		{"literal", "kit:\n  version: |\n    old\n", "block scalars"},
		{"quoted lines", "kit:\n  version: \"old\n    pin\"\n", "multiline"},
		{"plain lines", "kit:\n  version: old\n    pin\n", "multiline"},
		{"flow append", "{kit: {version: old}}\n", "block mapping"},
		{"end marker", "kit: {version: old}\n...\n", "document end marker"},
		{"internal comment", "kit: {version: old}\nmanaged:\n  - path: old # keep\n    sha256: old\n", "comments"},
		{"sequence anchor", "kit: {version: old}\nmanaged: &files []\n", "anchors"},
		{"sequence alias", "kit: {version: &pin old}\nmanaged: [{path: *pin, sha256: old}]\n", "anchors"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := bootstrap.ManagedPolicy([]byte(tc.input), "new", []policy.ManagedFile{{Path: "new", SHA256: "new"}})
			if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), tc.reason) || got != nil {
				t.Fatalf("unsupported splice = %q, %v", got, err)
			}
		})
	}
}

func TestManagedPolicyKeepsUnchangedValueCommentsAndClearsEntries(t *testing.T) {
	t.Parallel()
	before := []byte("kit: {version: old}\nmanaged:\n  - path: old # reviewed\n    sha256: old\n")
	unchanged := []policy.ManagedFile{{Path: "old", SHA256: "old"}}
	got, err := bootstrap.ManagedPolicy(before, "old", unchanged)
	if err != nil || !bytes.Equal(got, before) {
		t.Fatalf("unchanged value = %q, %v", got, err)
	}
	for _, clean := range []string{
		"kit: {version: old}\nmanaged:\n  - path: old\n    sha256: old\n",
		"kit: {version: old}\nmanaged:\n- path: old\n  sha256: old\n",
		"kit: {version: old}\n",
	} {
		got, err = bootstrap.ManagedPolicy([]byte(clean), "old", nil)
		if err != nil {
			t.Fatal(err)
		}
		decoded, decodeErr := policy.Decode(got)
		if decodeErr != nil || len(decoded.Managed) != 0 || !bytes.Contains(got, []byte("managed:")) {
			t.Fatalf("cleared value = %q, %v", got, decodeErr)
		}
	}
}
