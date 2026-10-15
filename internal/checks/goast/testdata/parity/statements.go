package parity

// manyStatements has 41 statements on 41 lines: the statements are
// reported and the line count is skipped.
func manyStatements(n int) int {
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	n++
	return n
}

// elseHeavy keeps the statements of its else branch out of the count, so
// only its 51 lines are reported.
func elseHeavy(n int) int {
	if n > 0 {
		n++
		n++
	} else {
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
		n--
	}
	return n
}
