package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"unicode"
	"unicode/utf8"
)

const (
	lineSeparatorRune      = 0x2028
	paragraphSeparatorRune = 0x2029
	separatorEscapeWidth   = len("u2028")
)

// CodexTrustKey is the key Codex keeps the trust of one handler under: the
// absolute path of the hooks file that declares it, the event in snake
// case and the zero-based indices of its group and of the handler in the
// group. A group another tool inserts before golaw's shifts the key, so
// the trust the person gave no longer applies to golaw's handler.
func CodexTrustKey(hooksPath, event string, group, handler int) string {
	return fmt.Sprintf("%s:%s:%d:%d", hooksPath, codexEventLabel(event), group, handler)
}

// codexEventLabel spells an event the way Codex labels it in a trust key
// and in the hashed identity: PreToolUse is pre_tool_use.
func codexEventLabel(event string) string {
	label := make([]rune, 0, len(event)+len(event)/2)
	for index, letter := range event {
		if unicode.IsUpper(letter) {
			if index > 0 {
				label = append(label, '_')
			}
			letter = unicode.ToLower(letter)
		}
		label = append(label, letter)
	}
	return string(label)
}

// codexHookEvent says golaw installs its handlers on event. Codex
// normalizes the handlers of its other events differently (a shorter
// timeout, no matcher, no context limit), so a hash for them is refused
// instead of guessed.
func codexHookEvent(event string) bool {
	return event == "PreToolUse" || event == "PostToolUse"
}

// codexDigest hashes identity as serde_json writes it. encoding/json
// sorts the keys of a map, and with HTML escaping off it writes what serde
// writes except U+2028 and U+2029, which it escapes and serde does not.
func codexDigest(identity map[string]any) (string, error) {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(identity); err != nil {
		return "", fmt.Errorf("encode codex hook identity: %w", err)
	}
	compact := bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))
	sum := sha256.Sum256(rawLineSeparators(compact))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func rawLineSeparators(encoded []byte) []byte {
	out := make([]byte, 0, len(encoded))
	for index := 0; index < len(encoded); index++ {
		if encoded[index] != '\\' || index+1 == len(encoded) {
			out = append(out, encoded[index])
			continue
		}
		if separator, ok := lineSeparator(encoded[index+1:]); ok {
			out = utf8.AppendRune(out, separator)
			index += separatorEscapeWidth
			continue
		}
		out = append(out, encoded[index], encoded[index+1])
		index++
	}
	return out
}

// lineSeparator reads the escape that follows a backslash in the output
// of encoding/json and answers the separator it stands for, if any.
func lineSeparator(escape []byte) (rune, bool) {
	switch string(escape[:min(len(escape), separatorEscapeWidth)]) {
	case "u2028":
		return lineSeparatorRune, true
	case "u2029":
		return paragraphSeparatorRune, true
	}
	return 0, false
}
