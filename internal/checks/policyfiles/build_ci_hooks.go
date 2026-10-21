package policyfiles

import (
	"fmt"
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/cristianargotti/golaw/internal/model"
)

// hookCommands judges a lefthook.yml only when present and only in the
// three rungs golaw arms, because every rung is optional (decision 10) and
// another git hook is the team's own; a heavier segment would make an
// optional rung slow, so it is the one finding.
func hookCommands(in model.Input) ([]model.Finding, error) {
	const file = "lefthook.yml"
	data, present, err := read(in, file)
	if err != nil || !present {
		return nil, err
	}
	var found []model.Finding
	for _, command := range hookFileCommands(file, data) {
		if !slices.ContainsFunc(buildStages(), func(stage buildStage) bool { return stage.name == command.hook }) {
			continue
		}
		for _, words := range hookSegments(command.run) {
			if hookHeavy(words) {
				message := fmt.Sprintf("command %s in %s runs %s; leave the check to make ci and use golaw hook %s", command.name, command.hook, strings.Join(words, " "), command.hook)
				found = append(found, finding(file, command.line, "hook-command-not-binary", message))
			}
		}
	}
	return found, nil
}

// hookFileCommands ignores unsupported syntax because optional hooks cannot limit CI.
func hookFileCommands(file string, data []byte) []lefthookCommand {
	commands, err := readLefthook(file, data)
	if err != nil {
		return nil
	}
	return commands
}

// hookHeavy recognizes only the checks make ci or an on-demand target
// carries, so a light command a team adds is never a finding.
func hookHeavy(words []string) bool {
	if len(words) < 2 {
		return false
	}
	name := strings.TrimSuffix(path.Base(strings.ReplaceAll(words[0], `\`, "/")), ".exe")
	switch name {
	case ciMakeCommand:
		return !strings.HasPrefix(words[1], "-") && !strings.Contains(words[1], "=") && words[1] != ""
	case "go":
		return slices.Contains([]string{"test", "vet", "build"}, words[1])
	case "golangci-lint":
		return words[1] == "run"
	case "gremlins":
		return words[1] == "unleash"
	default:
		return false
	}
}

// hookTokens is the state of the small tokenizer of a run value: literal
// quotes only, and a segment with an expansion marked invalid and dropped.
type hookTokens struct {
	segments [][]string
	words    []string
	word     []rune
	quote    rune
	invalid  bool
	active   bool
	comment  bool
}

// hookSegments splits a run value at the shell operators into the words of
// each command, so a compound is judged by its recognized segments.
func hookSegments(run string) [][]string {
	// Command substitutions can contain operators that are not command boundaries.
	if strings.Contains(run, "$(") || strings.ContainsRune(run, '`') {
		return nil
	}
	var tokens hookTokens
	characters := []rune(run)
	for index := 0; index < len(characters); index++ {
		character := characters[index]
		if tokens.quote == 0 && !tokens.comment && character == '&' && index+1 < len(characters) && characters[index+1] == '&' {
			tokens.segment()
			index++
			continue
		}
		tokens.character(character)
	}
	tokens.segment()
	return tokens.segments
}

func (tokens *hookTokens) character(character rune) {
	if tokens.comment {
		return
	}
	if tokens.quote != 0 {
		tokens.quoted(character)
		return
	}
	switch {
	case character == '\'' || character == '"':
		tokens.quote = character
		tokens.active = true
	case character == '#' && !tokens.active:
		tokens.comment = true
	case strings.ContainsRune("|;", character):
		tokens.segment()
	case unicode.IsSpace(character):
		tokens.finishWord()
	default:
		if strings.ContainsRune("$&<>()", character) {
			tokens.invalid = true
		}
		tokens.active = true
		tokens.word = append(tokens.word, character)
	}
}

func (tokens *hookTokens) quoted(character rune) {
	if character == tokens.quote {
		tokens.quote = 0
		return
	}
	if character == '$' && tokens.quote == '"' {
		tokens.invalid = true
	}
	tokens.word = append(tokens.word, character)
}

func (tokens *hookTokens) finishWord() {
	if tokens.active {
		tokens.words = append(tokens.words, string(tokens.word))
		tokens.word = tokens.word[:0]
		tokens.active = false
	}
}

func (tokens *hookTokens) segment() {
	tokens.finishWord()
	if !tokens.invalid && tokens.quote == 0 && len(tokens.words) > 0 {
		tokens.segments = append(tokens.segments, tokens.words)
	}
	tokens.words = nil
	tokens.invalid = false
}
