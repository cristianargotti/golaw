package app

import "context"

// Run propagates the caller's context.
func Run(ctx context.Context, load func(context.Context) error) error {
	return load(ctx)
}
