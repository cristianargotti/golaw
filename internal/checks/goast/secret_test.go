package goast_test

import (
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/goast"
	"github.com/cristianargotti/golaw/internal/model"
)

const clientSource = `package gateway

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

func (r *Redacted) LogValue() int { return 0 }
`

const pairSource = "package gateway\n\ntype Pair struct {\n\tSecret string\n}\n\nfunc (p Pair) LogValue() int { return 0 }\n"

func TestSecretFields(t *testing.T) {
	t.Parallel()
	res := scan(t, goast.SecretFields(), map[string]string{
		"internal/gw/client.go":      clientSource,
		"internal/gw/other.go":       pairSource,
		"internal/gw/client_test.go": "package gateway\n\ntype Fake struct {\n\tAPIKey string\n}\n",
	})
	expectCodes(t, res, "secret-field", "secret-field", "secret-field", "secret-field")
	expectLines(t, res, 6, 7, 12, 21)
	want := "credential in a bare string field Client.APIKey: give Client a LogValue that redacts it or type the field with a redacting type"
	if res.Findings[0].Message != want {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}

func TestSecretFieldsReadTheirSiblings(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", model.MaxReadBytes+1)
	res := scan(t, goast.SecretFields(), map[string]string{"internal/gw/client.go": clientSource, "internal/gw/zzz.go": big})
	expectUnmeasured(t, res, "read internal/gw/zzz.go")
	res = scan(t, goast.SecretFields(), map[string]string{"internal/gw/client.go": clientSource, "internal/gw/zzz.go": "package gateway\nfunc {"})
	expectUnmeasured(t, res, "parse internal/gw/zzz.go")
}
