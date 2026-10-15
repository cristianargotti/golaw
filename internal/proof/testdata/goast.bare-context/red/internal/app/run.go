package app

import "context"

// Run starts the use case with a fresh context instead of the caller's.
func Run(load func(context.Context) error) error {
	return load(context.Background())
}
