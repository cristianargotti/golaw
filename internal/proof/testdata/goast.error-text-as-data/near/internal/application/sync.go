package application

import "fmt"

type logs interface {
	Warn(msg string, args ...any)
}

// Sync logs the text and wraps the cause.
func Sync(logger logs, load func() error) error {
	if err := load(); err != nil {
		logger.Warn("sync failed", "cause", err.Error())
		return fmt.Errorf("sync: %w", err)
	}
	return nil
}
