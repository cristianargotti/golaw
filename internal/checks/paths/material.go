package paths

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

type secretMaterial struct{}

// SecretMaterial is paths.secret-material: a read event whose path names
// secret material, or whose links lead to a file that does, is denied by
// name; a source file named like a credential passes. Shell commands are
// judged by shell.secret-tokens with the same name list.
func SecretMaterial() model.Check { return secretMaterial{} }

func (secretMaterial) Spec() model.Spec { return spec("paths.secret-material", model.ScopeEvent) }

// Run judges the names of a read event and nothing else: the entry the
// name opens, then the file its links lead to.
func (secretMaterial) Run(_ context.Context, in model.Input) (model.Result, error) {
	if in.Event.Shape != model.ShapeRead {
		return model.Result{Status: model.NotApplicable("the secret name rule judges read events only")}, nil
	}
	res := model.Result{Status: model.Measured(), Scope: "1 read"}
	read := in.Event.Read
	var message string
	switch {
	case secretName(read.Path):
		message = fmt.Sprintf("the read names secret material (%s); secrets never enter the context", read.Path)
	case read.Real != "" && secretName(read.Real):
		message = fmt.Sprintf("the read leads through a link to secret material (%s); secrets never enter the context", read.Real)
	default:
		return res, nil
	}
	res.Findings = []model.Finding{{Code: "secret-material", Message: message, Severity: model.SeverityDeny}}
	return res, nil
}
