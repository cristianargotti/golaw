package ledgertest

// Fake keeps the answers it was built with.
type Fake struct {
	answers []string
}

// NewFake builds a fake from seven answers.
func NewFake(a, b, c, d, e, f, g string) *Fake {
	return &Fake{answers: []string{a, b, c, d, e, f, g}}
}
