package configs

import "os"

// Port reads the listening port from the environment with a default.
func Port() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return "8080"
}
