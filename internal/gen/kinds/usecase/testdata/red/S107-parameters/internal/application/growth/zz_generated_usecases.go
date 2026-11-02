package growth

// UseCases holds the ports it was built with.
type UseCases struct {
	ports []any
}

// NewUseCases builds the use cases from eight ports.
func NewUseCases(ledger, clock, mailer, ids, queue, store, cache, audit any) UseCases {
	return UseCases{ports: []any{ledger, clock, mailer, ids, queue, store, cache, audit}}
}
