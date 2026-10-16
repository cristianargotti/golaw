package gateway

import "log/slog"

// LogValue redacts the whole session.
func (s Session) LogValue() slog.Value {
	return slog.StringValue("[redacted]")
}
