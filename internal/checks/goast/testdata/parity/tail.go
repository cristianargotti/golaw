package parity

// tailComment ends with a comment that a blank line separates from the
// last statement: the comment map binds it to the next declaration, so
// funlen counts its two lines and reports 42.
func tailComment(n int) int {
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
	return n

	// first tail line
	// second tail line
}

// boundComment keeps its trailing comment on the line after the last
// statement, so the two lines are subtracted and 39 is counted.
func boundComment(n int) int {
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
	return n
	// first tail line
	// second tail line
}
