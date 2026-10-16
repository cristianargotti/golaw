package goast

import (
	"go/ast"
	"slices"

	"github.com/cristianargotti/golaw/internal/model"
)

const (
	codeEnvRead = "env-read"
	mainPackage = "main"
)

// selectorRule flags every use of pkg.Name for the listed names in the
// files the where predicate admits; three checks of the family share it.
type selectorRule struct {
	code    string
	pkg     string
	names   []string
	where   func(p string) bool
	roots   bool // a main package is a composition root wherever it sits
	message string
}

func (r selectorRule) judge(t *target, _ *state) ([]model.Finding, error) {
	if !r.where(t.file.Path) {
		return nil, nil
	}
	file, _, err := t.parsed()
	if err != nil {
		return nil, err
	}
	if r.roots && file.Name.Name == mainPackage {
		return nil, nil
	}
	var out []model.Finding
	ast.Inspect(file, func(n ast.Node) bool {
		if pkg, name, ok := selector(n); ok && pkg == r.pkg && slices.Contains(r.names, name) {
			out = append(out, t.finding(r.code, n.Pos(), pkg+"."+name+" "+r.message))
		}
		return true
	})
	return out, nil
}

// EnvRead is goast.env-read: configuration is read in one place, so an
// environment read outside configs/, cmd/ and the main packages (the
// composition roots, wherever the layout puts them) is a finding.
func EnvRead() model.Check {
	r := selectorRule{
		code:    codeEnvRead,
		pkg:     "os",
		names:   []string{"Getenv", "LookupEnv", "Environ"},
		where:   func(p string) bool { return !under(p, "configs/", cmdDir) },
		roots:   true,
		message: "reads the environment outside configs, cmd and a main package: add the knob to the configuration with a default",
	}
	return check{spec: spec("env-read"), judge: r.judge}
}
