package config

import (
	"crypto/rand"
	"encoding/hex"
)

// Config holds what a client needs.
type Config struct {
	Password string
	Token    string
	TokenURL string
}

// Default answers the configuration of a test with a value made at run time.
func Default() (Config, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return Config{}, err
	}
	return Config{Password: hex.EncodeToString(raw), Token: "", TokenURL: "https://auth.example.com/token"}, nil
}
