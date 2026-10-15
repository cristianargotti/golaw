package parity

func add(a, b int) int { return a + b }

// longPlain has 41 body lines over 11 statements, one line over the limit.
func longPlain(n int) int {
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	return n
}

// atLimit has exactly 40 body lines.
func atLimit(n int) int {
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n = add(
		n,
		1,
	)
	n++
	n++
	n++
	return n
}
