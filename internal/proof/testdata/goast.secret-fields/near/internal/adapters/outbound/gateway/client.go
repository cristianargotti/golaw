package gateway

import "log/slog"

// Secret prints redacted.
type Secret string

// LogValue redacts the secret.
func (Secret) LogValue() slog.Value {
	return slog.StringValue("[redacted]")
}

// Client types its credential and refers to a token by id.
type Client struct {
	APIKey  Secret
	TokenID string
	BaseURL string
}

// Session carries a token and redacts itself in session.go.
type Session struct {
	Token string
}
