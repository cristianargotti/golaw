package growth

// Deps holds every port the use cases consume.
type Deps struct {
	Ledger, Clock, Mailer, IDs, Queue, Store, Cache, Audit any
}

// UseCases holds the ports it was built with.
type UseCases struct {
	deps Deps
}

// NewUseCases builds the use cases from one Deps.
func NewUseCases(deps Deps) UseCases {
	return UseCases{deps: deps}
}
