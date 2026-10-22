package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

// rulesHeader states what a prefix cannot express, so a person who reads
// the file knows that the hook, not the file, judges those forms.
const rulesHeader = "# Generated from the hook's shared shell patterns.\n" +
	"# A prefix cannot express an option anywhere, git -C, or wrappers.\n" +
	"# User-scope rules also apply outside enrolled repositories.\n"

// prefixFamilies lists the two checks a prefix may come from (D26), read
// from the checks themselves: the ones the hook denies in every enrolled
// repository whatever the policy says. A policy option such as the push
// family would forbid outside the repositories what the hook allows
// inside them.
func prefixFamilies() []string {
	return []string{shell.NoVerify().Spec().ID, shell.CredentialPrint().Spec().ID}
}

// CodexRules writes the forbidden prefix rules of a Codex rules file from
// the prefix table of the shell family, one prefix_rule per prefix in
// order and nothing else: one rules file the host cannot parse drops every
// rule of the session, so the file carries no example and no other
// declaration, and an input the file could not express is refused with
// ErrSettings instead of written.
func CodexRules(prefixes []shell.Prefix) ([]byte, error) {
	lines := []string{rulesHeader}
	for index, prefix := range prefixes {
		line, err := prefixRule(prefix)
		if err != nil {
			return nil, fmt.Errorf("prefix %d: %w", index, err)
		}
		lines = append(lines, line)
	}
	return []byte(strings.Join(lines, "")), nil
}

func prefixRule(prefix shell.Prefix) (string, error) {
	if err := validatePrefix(prefix); err != nil {
		return "", err
	}
	positions := make([]string, 0, len(prefix.Words))
	for _, alternatives := range prefix.Words {
		position, err := rulePosition(alternatives)
		if err != nil {
			return "", err
		}
		positions = append(positions, position)
	}
	justification, err := ruleString(prefix.Reason)
	if err != nil {
		return "", err
	}
	return "prefix_rule(pattern=[" + strings.Join(positions, ",") +
		`], decision="forbidden", justification=` + justification + ")\n", nil
}

func validatePrefix(prefix shell.Prefix) error {
	if !slices.Contains(prefixFamilies(), prefix.Check) {
		return fmt.Errorf("check %q is not a prefix family: %w", prefix.Check, ErrSettings)
	}
	if len(prefix.Words) == 0 {
		return fmt.Errorf("prefix without words: %w", ErrSettings)
	}
	for _, alternatives := range prefix.Words {
		if len(alternatives) == 0 {
			return fmt.Errorf("prefix position without alternatives: %w", ErrSettings)
		}
		if slices.Contains(alternatives, "") {
			return fmt.Errorf("prefix with an empty word: %w", ErrSettings)
		}
	}
	if prefix.Reason == "" {
		return fmt.Errorf("prefix without a reason: %w", ErrSettings)
	}
	return nil
}

// rulePosition writes one argument position as the host spells it: a
// string for one word, an array for alternatives.
func rulePosition(alternatives []string) (string, error) {
	encoded := make([]string, 0, len(alternatives))
	for _, word := range alternatives {
		text, err := ruleString(word)
		if err != nil {
			return "", err
		}
		encoded = append(encoded, text)
	}
	if len(encoded) == 1 {
		return encoded[0], nil
	}
	return "[" + strings.Join(encoded, ",") + "]", nil
}

// ruleString encodes a string as JSON without the HTML escapes, a literal
// the Starlark reader of the host accepts as written.
func ruleString(text string) (string, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(text); err != nil {
		return "", fmt.Errorf("encode rule string: %w", err)
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}
