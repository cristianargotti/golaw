package plan

import "context"

// Root answers the context of the composition root.
func Root() context.Context {
	return context.Background()
}
