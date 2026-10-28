package lint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/runner"
)

// Format names the machine output a tool prints under the flag the kit
// passes it.
type Format string

// The machine formats golaw reads.
const (
	FormatESLint     Format = "eslint-json"       // eslint --format json
	FormatStylelint  Format = "stylelint-json"    // stylelint --formatter json
	FormatRuff       Format = "ruff-json"         // ruff check --output-format json
	FormatRuffFormat Format = "ruff-format-check" // ruff format --check
	FormatPyright    Format = "pyright-json"      // pyright --outputjson
	FormatHadolint   Format = "hadolint-json"     // hadolint --format json
	FormatShellcheck Format = "shellcheck-json1"  // shellcheck --format json1
	FormatShfmt      Format = "shfmt-diff"        // shfmt -d
	FormatCfnLint    Format = "cfn-lint-json"     // cfn-lint --format json
	FormatCheckov    Format = "checkov-json"      // checkov --output json
	FormatYamllint   Format = "yamllint-parsable" // yamllint --format parsable
)

// parser reads one format: accepts tells the exit statuses the tool uses
// for a run that reported, every other status being its own failure.
type parser struct {
	accepts func(code int) bool
	parse   func(out runner.Output, tool, root string) ([]Issue, error)
}

func parsers() map[Format]parser {
	return map[Format]parser{
		FormatESLint:     {codes(0, 1), parseESLint},
		FormatStylelint:  {codes(0, 2), parseStylelint},
		FormatRuff:       {codes(0, 1), parseRuff},
		FormatRuffFormat: {codes(0, 1, 2), parseRuffFormat},
		FormatPyright:    {codes(0, 1), parsePyright},
	}
}

// Parse reads the output of one tool run in format. An unknown format or
// output it cannot read is ErrFormat with no issue, an exit status the tool
// keeps for its own failure is ErrToolFailed, and a file the tool could not
// parse is ErrUnparsed beside the issues of the other files.
func Parse(format Format, tool string, out runner.Output, root string) ([]Issue, error) {
	p, ok := parsers()[format]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrFormat, format)
	}
	if !p.accepts(out.Code) {
		return nil, toolFailed(tool, out)
	}
	issues, err := p.parse(out, tool, root)
	if errors.Is(err, ErrFormat) {
		return nil, err
	}
	return issues, err
}

func toolFailed(tool string, out runner.Output) error {
	return fmt.Errorf("%w: %s exited %d: %s", ErrToolFailed, tool, out.Code, firstLine(out.Stderr))
}

func codes(accepted ...int) func(int) bool {
	return func(code int) bool { return slices.Contains(accepted, code) }
}

func unparsed(tool string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	names := slices.Compact(slices.Sorted(slices.Values(paths)))
	return fmt.Errorf("%w: %s: %s", ErrUnparsed, tool, strings.Join(names, ", "))
}

func firstLine(data []byte) string {
	line, _, _ := strings.Cut(strings.TrimSpace(string(data)), "\n")
	return strings.TrimSpace(line)
}

// decodeJSON is lenient about unknown keys, because a tool adds keys between
// minors and only the keys read here bind.
func decodeJSON(data []byte, into any) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("%w: no output", ErrFormat)
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}
	return nil
}
