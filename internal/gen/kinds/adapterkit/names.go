package adapterkit

import (
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
)

// zerosOf names one variable per result before the error, skipping the
// parameter names so the declarations never shadow an argument.
func zerosOf(results, params []genkind.Param) []genkind.Param {
	out := make([]genkind.Param, 0, len(results)-1)
	next := 1
	for _, r := range results[:len(results)-1] {
		name := zeroName(next)
		for slices.ContainsFunc(params, func(p genkind.Param) bool { return p.Name == name }) {
			next++
			name = zeroName(next)
		}
		next++
		out = append(out, genkind.Param{Name: name, Type: r.Type})
	}
	return out
}

func zeroName(n int) string {
	if n == 1 {
		return "zero"
	}
	return "zero" + strconv.Itoa(n)
}

// inputsOf are the parameters after the context, renamed where a red test
// declares the same name itself.
func inputsOf(params, zeros []genkind.Param) []genkind.Param {
	out := make([]genkind.Param, 0, len(params)-1)
	for _, p := range params[1:] {
		if reserved(p.Name) || slices.ContainsFunc(zeros, func(z genkind.Param) bool { return z.Name == p.Name }) {
			p.Name += "Value"
		}
		out = append(out, p)
	}
	return out
}

// reserved reports a name a red test declares itself, so an input variable
// named like one is renamed instead of shadowing it.
func reserved(name string) bool {
	switch name {
	case "t", "err", "db", "adapter", "server":
		return true
	}
	return false
}

// docLines keeps the contract's doc and opens it with the method name when
// it does not, because the lint asks every exported method for that doc.
func docLines(m genkind.Method, port string) []string {
	text := strings.TrimSpace(m.Doc)
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	if text == "" || !strings.HasPrefix(text, m.Name) {
		lines = append([]string{m.Name + " implements the " + port + " port."}, lines...)
	}
	return lines
}
