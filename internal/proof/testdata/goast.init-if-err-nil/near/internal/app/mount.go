package app

import "strconv"

// Mount parses with a fallback and names the fallback of the resolver.
func Mount(raw string, resolve func() (string, error), use func(string)) int {
	if dir, err := resolve(); err == nil { // fallback: the working directory serves when the root is unset
		use(dir)
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n
	}
	return 0
}
