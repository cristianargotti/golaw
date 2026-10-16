package shell_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

func TestSecretTokens(t *testing.T) {
	t.Parallel()
	denies(t, shell.SecretTokens(), "secret-token", map[string]bool{
		"cat /work/app/.env":                              true,
		"cat internal/credentials.go":                     false,
		"kubectl --kubeconfig=./prod.kubeconfig get pods": true,
		"cat <deploy.pem":                                 true,
		"source ~/.zshrc":                                 true,
		"cat $HOME/.aws/credentials":                      true,
		"ls":                                              false,
		"cat .env.example":                                false,
		"cat docs/secrets.md":                             false,
		"bash -c 'cat secrets/db.yaml'":                   true,
		"go build ./...":                                  false,
		"cat id_rsa":                                      true,
		"ssh -i id_ed25519 host":                          true,
		"scp ID_ECDSA backup:keys":                        true,
		"cp id_dsa --target-directory=out":                true,
		"cat id_rsa.pub":                                  false,
		"grep -rn credentials .":                          false,
		"rg kubeconfig docs":                              false,
		"git checkout -b fix-credentials":                 false,
		"git commit -m 'fix: rotate the credentials'":     false,
		"git commit -m 'docs: say where id_rsa lives'":    false,
		"go test ./internal/secrets/...":                  false,
		"go test ./internal/secret/...":                   false,
		"ls internal/secrets/":                            false,
		"cat deploy/secrets/token.txt":                    true,
		"cat charts/app/templates/secret.yaml":            false,
		"cat config/credentials.example.json":             false,
		"cat internal/secrets/testdata/payload.json":      true,
		"cat secrets/testdata/token.txt":                  true,
		"cat internal/secrets/testdata/fixture.go":        false,
	})
	res := judge(t, shell.SecretTokens(), "cat /work/app/.env")
	if res.Findings[0].Message != "the command names secret material (/work/app/.env); secrets never enter the context" {
		t.Errorf("message = %q", res.Findings[0].Message)
	}
}

// nameLiterals returns the string literals of the secretNames function and
// of the sourceFile and exampleFile patterns of a family's secret.go, the
// data the two families must carry alike.
func nameLiterals(t *testing.T, source []byte) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "secret.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var literals []string
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.ImportSpec:
			return false
		case *ast.FuncDecl:
			return n.Name.Name == "secretNames"
		case *ast.ValueSpec:
			return n.Names[0].Name == "sourceFile" || n.Names[0].Name == "exampleFile"
		case *ast.BasicLit:
			if n.Kind == token.STRING {
				literals = append(literals, n.Value)
			}
		}
		return true
	})
	return literals
}

// sourceOf reads a source file of the checks tree.
func sourceOf(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestSecretNamesMatchThePathsFamily(t *testing.T) {
	t.Parallel()
	own := nameLiterals(t, sourceOf(t, "secret.go"))
	paths := sourceOf(t, "../paths/secret.go")
	if len(own) < 20 || !slices.Equal(own, nameLiterals(t, paths)) {
		t.Fatalf("the shell name list differs from the paths one:\n%q\n%q", own, nameLiterals(t, paths))
	}
	drifted := strings.Replace(string(paths), "`\\.vault-token`", "`\\.vault-tokens`", 1)
	if drifted == string(paths) || slices.Equal(own, nameLiterals(t, []byte(drifted))) {
		t.Fatal("a drifted name list compared equal")
	}
}
