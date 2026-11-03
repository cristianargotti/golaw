package rulesdoc

import "errors"

// The failure modes of a lookup: the caller prints the reason and the
// names it can ask for instead.
var (
	ErrNoRule      = errors.New("no rule matches")
	ErrNoChecklist = errors.New("no checklist section matches")
	ErrTwoQueries  = errors.New("give a rule id or --checklist, not both")
)
