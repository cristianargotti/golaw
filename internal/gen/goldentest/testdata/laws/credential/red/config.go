package config

// Config holds what a client needs.
type Config struct {
	Password string
}

// Default answers the configuration of a test.
func Default() Config {
	return Config{Password: "fixture"}
}
