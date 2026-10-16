package app

type noop struct{}

// Flush does nothing and does not say why.
func (noop) Flush() {}
