package app

type noop struct{}

// Flush does nothing.
func (noop) Flush() {
	// nothing is buffered: the writer flushes on every write
}
