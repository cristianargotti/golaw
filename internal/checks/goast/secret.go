package goast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"

	"github.com/cristianargotti/golaw/internal/model"
)

const codeSecretField = "secret-field"

var (
	// credentialName anchors at the end of the name, plural included, so
	// APIKeys and DBPassword hold a credential and TokenSource does not.
	credentialName = regexp.MustCompile(`(?i)(api_?key|secret|token|password|passphrase|credential)s?$`)
	// credentialReference matches the names that point at a credential
	// kept elsewhere, a TokenID or a SecretName, which hold no secret.
	credentialReference = regexp.MustCompile(`(?i)(id|hash|name|path|url)$`)
)

// SecretFields is goast.secret-fields: a credential never sits in a bare
// string field. A credential-named string field of a package-level struct
// is a finding unless the struct implements slog.LogValuer somewhere in
// its package; a field whose name ends in id, hash, name, path or url only
// refers to a credential and passes.
func SecretFields() model.Check {
	return check{spec: spec("secret-fields"), judge: judgeSecretFields}
}

// credentialField keeps its owner, because the redaction that exempts a
// field belongs to its struct.
type credentialField struct {
	owner string
	name  string
	pos   token.Pos
}

func judgeSecretFields(t *target, s *state) ([]model.Finding, error) {
	file, _, err := t.parsed()
	if err != nil {
		return nil, err
	}
	var out []model.Finding
	for _, field := range credentialFields(file) {
		redacted, err := s.redacting(t.dir())
		if err != nil {
			return nil, err
		}
		if redacted[field.owner] {
			continue
		}
		message := fmt.Sprintf("credential in a bare string field %s.%s: give %s a LogValue that redacts it or type the field with a redacting type",
			field.owner, field.name, field.owner)
		out = append(out, t.finding(codeSecretField, field.pos, message))
	}
	return out, nil
}

// credentialFields reads the package-level structs only, the types a
// LogValue method can be declared on.
func credentialFields(file *ast.File) []credentialField {
	var out []credentialField
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, s := range gen.Specs {
			out = append(out, structCredentials(s)...)
		}
	}
	return out
}

func structCredentials(s ast.Spec) []credentialField {
	spec, ok := s.(*ast.TypeSpec)
	if !ok {
		return nil
	}
	st, ok := spec.Type.(*ast.StructType)
	if !ok {
		return nil
	}
	var out []credentialField
	for _, field := range st.Fields.List {
		if !isIdent(field.Type, "string") {
			continue
		}
		for _, name := range field.Names {
			if isCredential(name.Name) {
				out = append(out, credentialField{owner: spec.Name.Name, name: name.Name, pos: name.Pos()})
			}
		}
	}
	return out
}

func isCredential(name string) bool {
	return credentialName.MatchString(name) && !credentialReference.MatchString(name)
}

// redacting reads every non-test Go file of the directory once per run,
// because the slog.LogValuer of a struct may sit in any file of its
// package.
func (s *state) redacting(dir string) (map[string]bool, error) {
	if types, ok := s.redacted[dir]; ok {
		return types, nil
	}
	types := map[string]bool{}
	for _, f := range s.siblings(dir) {
		if f.Class == model.ClassTest {
			continue
		}
		src, err := s.in.Snapshot.Read(f.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f.Path, err)
		}
		file, err := parseSource(f.Path, src)
		if err != nil {
			return nil, err
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && isLogValuer(fn) {
				types[typeName(fn.Recv.List[0].Type)] = true
			}
		}
	}
	s.redacted[dir] = types
	return types, nil
}

// isLogValuer reports a method with the signature of slog.LogValuer,
// LogValue() slog.Value: slog calls no other shape, so a LogValue with
// parameters or another result redacts nothing.
func isLogValuer(fn *ast.FuncDecl) bool {
	if fn.Recv == nil || fn.Name.Name != "LogValue" || fn.Type.Params.NumFields() != 0 || fn.Type.Results.NumFields() != 1 {
		return false
	}
	pkg, name, ok := selector(fn.Type.Results.List[0].Type)
	return ok && pkg == "slog" && name == "Value"
}

// parseSource gives a sibling a file set of its own, since no finding
// ever points into a sibling.
func parseSource(name string, src []byte) (*ast.File, error) {
	file, err := parser.ParseFile(token.NewFileSet(), name, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, parseError(name, err)
	}
	return file, nil
}
