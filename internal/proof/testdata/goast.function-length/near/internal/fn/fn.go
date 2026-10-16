package fn

func add(a, b int) int { return a + b }

// walk has 40 code lines and four comment lines: the comments are ignored.
func walk(n int) int {
	// the first leg
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
	// the second leg
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
	// the last steps
	n++
	n++
	n++
	// and the answer
	return n
}
