package app

import "os"

// Port reads the listening port from the environment.
func Port() string {
	return os.Getenv("PORT")
}
