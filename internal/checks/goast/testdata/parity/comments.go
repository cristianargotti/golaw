package parity

// commentedLong has 40 code lines and 8 comment lines: 48 raw, 40 counted.
func commentedLong(n int) int {
	n = add(
		n,
		1,
	)
	// step 1 of the walk
	n = add(
		n,
		1,
	)
	// step 2 of the walk
	n = add(
		n,
		1,
	)
	// step 3 of the walk
	n = add(
		n,
		1,
	)
	// step 4 of the walk
	n = add(
		n,
		1,
	)
	// step 5 of the walk
	n = add(
		n,
		1,
	)
	// step 6 of the walk
	n = add(
		n,
		1,
	)
	// step 7 of the walk
	n = add(
		n,
		1,
	)
	// step 8 of the walk
	n = add(
		n,
		1,
	)
	n++
	n++
	n++
	return n
}

// trailingComments has 42 code lines, two of them with a trailing comment,
// which funlen subtracts like any comment: 40 counted.
func trailingComments(n int) int {
	n++ // one
	n++ // two
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

// blockComment has 41 code lines and a three-line block comment: 44 raw,
// one entry subtracted, 43 counted.
func blockComment(n int) int {
	/*
	   the walk below is deliberate
	*/
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
