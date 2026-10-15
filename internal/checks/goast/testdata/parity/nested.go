package parity

// nestedStatements descends the for, range, type switch, block, defer and
// go statements: 43 statements in all, reported as statements.
func nestedStatements(xs []any) int {
	n := 0
	for i := 0; i < 3; i++ {
		n++
		n++
		n++
		n++
		n++
	}
	for range xs {
		n++
		n++
		n++
		n++
		n++
	}
	switch v := xs[0].(type) {
	case int:
		n += v
		n += v
		n += v
		n += v
		n += v
	default:
		n++
		n++
		n++
		n++
		n++
	}
	{
		n++
		n++
		n++
		n++
		n++
		n++
	}
	defer func() {
		n++
		n++
		n++
		n++
	}()
	go func() {
		n++
		n++
		n++
		n++
	}()
	return n
}

// shortNested stays at 39 statements over 46 lines, so only its lines are
// reported.
func shortNested(xs []any) int {
	n := 0
	for i := 0; i < 3; i++ {
		n++
		n++
		n++
		n++
		n++
	}
	for range xs {
		n++
		n++
		n++
		n++
		n++
	}
	switch v := xs[0].(type) {
	case int:
		n += v
		n += v
		n += v
		n += v
		n += v
	default:
		n++
		n++
		n++
		n++
		n++
	}
	{
		n++
		n++
		n++
		n++
		n++
		n++
	}
	defer func() {
		n++
		n++
	}()
	go func() {
		n++
		n++
	}()
	return n
}
