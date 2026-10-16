package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
)

const clientSource = `package gateway

import "log/slog"

type Secret string

type Client struct {
	APIKey      string
	apiKey      string
	Token       Secret
	TokenID     string
	SecretName  string
	KeyPath     string
	Passwords   string
	User, Pass  string
	Credentials []string
	Note        string
	Passphrase  *string
}

type (
	Session struct {
		Token string
	}
	Plain int
)

type Redacted struct {
	Password string
}

func (r *Redacted) LogValue() slog.Value { return slog.StringValue("[redacted]") }
`

const pairSource = `package gateway

import "log/slog"

type Pair struct {
	Secret string
}

func (p Pair) LogValue() slog.Value { return slog.StringValue("[redacted]") }
`

func TestSecretFields(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SecretFields(), map[string]string{
		"internal/gw/client.go":      clientSource,
		"internal/gw/other.go":       pairSource,
		"internal/gw/client_test.go": "package gateway\n\ntype Fake struct {\n\tAPIKey string\n}\n",
	})
	expectCodes(t, res, "secret-field", "secret-field", "secret-field", "secret-field")
	expectLines(t, res, 8, 9, 14, 23)
	want := "credential in a bare string field Client.APIKey: give Client a LogValue that redacts it or type the field with a redacting type"
	if res.Findings[0].Message != want {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}

// valuerSource gives a credential field to four structs and a LogValue
// to each: only the last one has the slog.LogValuer signature.
const valuerSource = `package gateway

import "log/slog"

type Counted struct {
	Password string
}

func (c Counted) LogValue() int { return 0 }

type Leveled struct {
	Token string
}

func (l Leveled) LogValue(level int) slog.Value { return slog.Value{} }

type Paired struct {
	Secret string
}

func (p Paired) LogValue() (slog.Value, error) { return slog.Value{}, nil }

type Valued struct {
	Passphrase string
}

func (v *Valued) LogValue() (value slog.Value) { return slog.StringValue("[redacted]") }

func LogValue() slog.Value { return slog.Value{} }
`

func TestSecretFieldsExemptOnlyASlogLogValuer(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SecretFields(), map[string]string{"internal/gw/valuer.go": valuerSource})
	expectCodes(t, res, "secret-field", "secret-field", "secret-field")
	expectLines(t, res, 6, 12, 18)
}

func TestSecretFieldsReadTheirSiblings(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", model.MaxReadBytes+1)
	res := scan(t, goast.SecretFields(), map[string]string{"internal/gw/client.go": clientSource, "internal/gw/zzz.go": big})
	expectUnmeasured(t, res, "read internal/gw/zzz.go")
	res = scan(t, goast.SecretFields(), map[string]string{"internal/gw/client.go": clientSource, "internal/gw/zzz.go": "package gateway\nfunc {"})
	expectUnmeasured(t, res, "parse internal/gw/zzz.go")
}
