package goast

import "github.com/cristianargotti/golaw/internal/model"

const codeWallClock = "wall-clock"

// WallClock is goast.wall-clock: time enters the domain and the use cases
// through a parameter or a clock function, never from the process clock.
func WallClock() model.Check {
	r := selectorRule{
		code:    codeWallClock,
		pkg:     "time",
		names:   []string{"Now", "Since", "Until"},
		where:   func(p string) bool { return under(p, "internal/domain/", "internal/application/") },
		message: "reads the wall clock in the domain or a use case: take now as a parameter or a clock function",
	}
	return check{spec: spec("wall-clock"), judge: r.judge}
}
