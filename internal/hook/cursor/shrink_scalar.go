package cursor

import (
	"errors"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/model"
)

// The places of a number scan after the byte it just read; a number may end
// only after a digit of its integer, fraction or exponent.
const (
	afterMinus = iota
	afterZero
	inInteger
	afterPoint
	inFraction
	afterExponent
	afterExponentSign
	inExponent
)

// number copies a number whose first byte was read, checking its grammar
// one byte at a time, so a run of digits is never held whole. The byte that
// ends it is carried to the grammar. A number longer than the shorter bound
// is kept as 0: no field a route judges by is a number, and 0 keeps its
// type for the decoders that refuse a field of the wrong one.
func (r *reducer) number(first byte) error {
	place := afterMinus
	if first != '-' {
		place = digitStep(afterMinus, first)
	}
	start, size := len(r.out), 1
	r.out = append(r.out, first)
	for {
		c, err := r.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return cut(err)
		}
		step := numberStep(place, c)
		if step < 0 {
			r.carry, r.carried = c, true
			break
		}
		place, size = step, size+1
		if size <= keptText {
			r.out = append(r.out, c)
		}
	}
	if !ends(place) {
		return malformed("a number")
	}
	if size > keptText {
		r.out = append(r.out[:start], '0')
	}
	return nil
}

// numberStep is the place after c, or -1 when c does not continue the
// number from place.
func numberStep(place int, c byte) int {
	switch {
	case isDigit(c):
		return digitStep(place, c)
	case c == '.' && (place == afterZero || place == inInteger):
		return afterPoint
	case (c == 'e' || c == 'E') && ends(place) && place != inExponent:
		return afterExponent
	case (c == '+' || c == '-') && place == afterExponent:
		return afterExponentSign
	default:
		return -1
	}
}

// digitStep is the place after a digit: a leading zero takes no digit after
// it, and a digit after a point or an exponent mark begins its part.
func digitStep(place int, c byte) int {
	switch place {
	case afterMinus:
		if c == '0' {
			return afterZero
		}
		return inInteger
	case afterZero:
		return -1
	case inInteger:
		return inInteger
	case afterPoint, inFraction:
		return inFraction
	default:
		return inExponent
	}
}

// ends reports a place a number may end at.
func ends(place int) bool {
	return place == afterZero || place == inInteger || place == inFraction || place == inExponent
}

// literal copies true, false or null, whose first byte was read.
func (r *reducer) literal(first byte) error {
	word := "null"
	switch first {
	case 't':
		word = "true"
	case 'f':
		word = "false"
	}
	for i := 1; i < len(word); i++ {
		c, err := r.next()
		if err != nil {
			return cut(err)
		}
		if c != word[i] {
			return malformed("a literal")
		}
	}
	r.out = append(r.out, word...)
	return nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// malformed refuses an input that breaks the grammar, naming where and
// never what it held.
func malformed(where string) error {
	return fmt.Errorf("%w: an input over 8 MiB is not valid JSON at %s", model.ErrCannotJudge, where)
}

// cut refuses an input that ends inside the document or cannot be read.
func cut(err error) error {
	if errors.Is(err, io.EOF) {
		return malformed("its end, inside the document")
	}
	return fmt.Errorf("%w: %w", model.ErrCannotJudge, err)
}
