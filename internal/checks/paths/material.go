package paths

import (
	"context"
	"fmt"

	"github.com/cristianargotti/golaw/internal/model"
)

type secretMaterial struct{}

// SecretMaterial is paths.secret-material: a read event whose path names
// secret material is denied by name; a source file named like a credential
// passes. Shell commands are judged by shell.secret-tokens with the same
// name list.
func SecretMaterial() model.Check { return secretMaterial{} }

func (secretMaterial) Spec() model.Spec { return spec("paths.secret-material", model.ScopeEvent) }

// Run judges the path of a read event and nothing else.
func (secretMaterial) Run(_ context.Context, in model.Input) (model.Result, error) {
	if in.Event.Shape != model.ShapeRead {
		return model.Result{Status: model.NotApplicable("the secret name rule judges read events only")}, nil
	}
	res := model.Result{Status: model.Measured(), Scope: "1 read"}
	if secretName(in.Event.Read.Path) {
		message := fmt.Sprintf("the read names secret material (%s); secrets never enter the context", in.Event.Read.Path)
		res.Findings = []model.Finding{{Code: "secret-material", Message: message, Severity: model.SeverityDeny}}
	}
	return res, nil
}
