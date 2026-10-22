package cursor_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/hook/cursor"
	"github.com/cristianargotti/golaw/internal/hooktest"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

var errCursorInput = errors.New("input unavailable")

func TestCursorOversizedGrammar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value string
		bad   bool
	}{
		{"near empty string", `""`, false},
		{"near escaped quote", `"\""`, false},
		{"near escaped backslash", `"\\"`, false},
		{"near escaped slash", `"\/"`, false},
		{"near backspace", `"\b"`, false},
		{"near form feed", `"\f"`, false},
		{"near newline", `"\n"`, false},
		{"near carriage return", `"\r"`, false},
		{"near tab", `"\t"`, false},
		{"near unicode digits", `"a"`, false},
		{"near unicode mixed hex", `"¯"`, false},
		{"near surrogate pair", `"😀"`, false},
		{"near lone high surrogate", `"\uD800"`, false},
		{"near lone low surrogate", `"\uDC00"`, false},
		{"red unknown escape", `"\q"`, true},
		{"red invalid unicode digit", `"\u00q0"`, true},
		{"red incomplete unicode", `"\u12`, true},
		{"red incomplete string", `"value`, true},
		{"red control byte", "\"a\nb\"", true},
		{"near zero", `0`, false},
		{"near negative zero", `-0`, false},
		{"near integer", `12`, false},
		{"near negative", `-12`, false},
		{"near fraction", `0.125`, false},
		{"near signed fraction", `-12.5`, false},
		{"near exponent", `1e2`, false},
		{"near positive exponent", `1E+2`, false},
		{"near negative exponent", `1e-2`, false},
		{"red sign only", `-`, true},
		{"red leading zero", `01`, true},
		{"red fraction without digit", `1.`, true},
		{"red exponent without digit", `1e`, true},
		{"red exponent sign only", `1e+`, true},
		{"red positive sign", `+1`, true},
		{"red repeated fraction", `1..2`, true},
		{"red repeated exponent sign", `1e--1`, true},
		{"near true", `true`, false},
		{"near false", `false`, false},
		{"near null", `null`, false},
		{"red misspelled true", `truX`, true},
		{"red misspelled false", `falsX`, true},
		{"red misspelled null", `nulX`, true},
		{"near empty array", `[]`, false},
		{"near empty object", `{}`, false},
		{"near nested containers", `[{"a":[true,false,null]},{}]`, false},
		{"near spaced tokens", " [ \n { \t\"a\" \r: [ 1 , 2 ] } ] ", false},
		{"red array trailing comma", `[1,]`, true},
		{"red object trailing comma", `{"a":1,}`, true},
		{"red missing colon", `{"a" 1}`, true},
		{"red missing comma", `[1 2]`, true},
		{"red unquoted key", `{a:1}`, true},
		{"red mismatched close", `[1}`, true},
		{"red missing value", `{"a":}`, true},
		{"red invalid value", `?`, true},
		{"red incomplete array", `[`, true},
		{"red incomplete object", `{`, true},
		{"near nesting limit", strings.Repeat("[", 9999) + "0" + strings.Repeat("]", 9999), false},
		{"red above nesting limit", strings.Repeat("[", 10000) + "0" + strings.Repeat("]", 10000), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			data := curGrammarInput(t, test.value)
			request, err := curReducedRequest(t, data)
			curAssertReducedRead(t, request, err, test.bad)
		})
	}
}

func curGrammarInput(t *testing.T, value string) []byte {
	t.Helper()
	base := bytes.TrimSpace(curBytes(t, "proof/testdata/read/near/payload.json"))
	return []byte(string(base[:len(base)-1]) + `,"future":` + value + `}`)
}

func curReducedRequest(t *testing.T, data []byte) (hook.Request, error) {
	t.Helper()
	root := hooktest.Repository(t)
	parsed, err := cursor.Oversized(bytes.NewReader(hooktest.Payload(data, root)))
	if err != nil {
		return hook.Request{}, fmt.Errorf("oversized: %w", err)
	}
	request, err := parsed.Decode(hook.Base{Root: root}, proof.NewTree(nil))
	if err != nil {
		return hook.Request{}, fmt.Errorf("decode: %w", err)
	}
	return request, nil
}

func curAssertReducedRead(t *testing.T, request hook.Request, err error, bad bool) {
	t.Helper()
	if bad {
		if !errors.Is(err, model.ErrCannotJudge) {
			t.Fatalf("expected cannot judge, got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if request.Event.Read == nil || request.Event.Read.Path != "main.go" ||
		len(request.Content) != 0 {
		t.Fatalf("reduced read = %+v", request)
	}
}

func TestCursorOversizedReaderFailure(t *testing.T) {
	t.Parallel()
	base := bytes.TrimSpace(curBytes(t, "proof/testdata/read/near/payload.json"))
	for _, test := range []struct {
		name  string
		input io.Reader
		bad   bool
	}{
		{"red first read", iotest.ErrReader(errCursorInput), true},
		{"red interrupted object", io.MultiReader(bytes.NewReader(base[:len(base)-1]),
			iotest.ErrReader(errCursorInput)), true},
		{"red final read", io.MultiReader(bytes.NewReader(base),
			iotest.ErrReader(errCursorInput)), true},
		{"near complete reader", bytes.NewReader(base), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := cursor.Oversized(test.input)
			if test.bad && !errors.Is(err, model.ErrCannotJudge) {
				t.Fatalf("expected cannot judge, got %v", err)
			}
			if !test.bad && err != nil {
				t.Fatal(err)
			}
		})
	}
}
