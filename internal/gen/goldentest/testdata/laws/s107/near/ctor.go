package ctor

// Config holds the parts of one value.
type Config struct {
	A, B, C, D, E, F, G, H int
}

// New builds one value from seven parts.
func New(a, b, c, d, e, f, g int) int {
	return a + b + c + d + e + f + g
}

// FromConfig builds one value from its configuration.
func FromConfig(cfg Config) int {
	return cfg.A + cfg.H
}
