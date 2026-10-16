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

// TestSecretTokensReadDirectoriesAndPackagePatterns judges a directory
// operand like a file, since a recursive read of a directory under a
// secrets directory reads what lies below it, and leaves a Go package
// pattern to the go tool and its linters alone, which build or analyze
// the packages it names and print no file.
func TestSecretTokensReadDirectoriesAndPackagePatterns(t *testing.T) {
	t.Parallel()
	denies(t, shell.SecretTokens(), "secret-token", map[string]bool{
		"grep -R . deploy/secrets/db/":                             true,
		"ls deploy/secrets/db/":                                    true,
		"cat ./deploy/secrets/...":                                 true,
		"go test ./internal/secrets/... && cat deploy/secrets/db/": true,
		"go test ./internal/secrets/...":                           false,
		"go test ./internal/secret/...":                            false,
		"go vet ./internal/secrets/...":                            false,
		"golangci-lint run ./internal/secrets/...":                 false,
		"env GOFLAGS=-count=1 go test ./deploy/secrets/...":        false,
		"ls internal/secrets/":                                     false,
	})
}

// TestSecretTokensSkipOnlyPackageOperands leaves a pattern ending in /...
// alone only where the go tool or a linter reads it as packages: after the
// flags of a package subcommand, with the value of a flag skipped, as the
// one package of go run and as the list of -coverpkg; a redirection and its
// file, the value of a flag, what go run and go test pass on to the
// program and the words of a subcommand that reads files are judged.
func TestSecretTokensSkipOnlyPackageOperands(t *testing.T) {
	t.Parallel()
	denies(t, shell.SecretTokens(), "secret-token", map[string]bool{
		"go run ./cat.go ./deploy/secrets/...":                                    true,
		"go run ./x.go < ./deploy/secrets/...":                                    true,
		"go run ./x.go <./deploy/secrets/...":                                     true,
		"go build ./... 2>./deploy/secrets/...":                                   true,
		"go vet ./... > ./deploy/secrets/...":                                     true,
		"go test ./x -args ./deploy/secrets/...":                                  true,
		"go test ./x -run X ./deploy/secrets/...":                                 true,
		"go test -mine v ./deploy/secrets/...":                                    true,
		"go test -- ./deploy/secrets/...":                                         true,
		"go tool asm ./deploy/secrets/...":                                        true,
		"go mod edit ./deploy/secrets/...":                                        true,
		"go -C":                                                                   false,
		"go build -overlay ./deploy/secrets/... ./...":                            true,
		"go test -coverpkg=./x/...,./deploy/secrets/token ./...":                  true,
		"golangci-lint run -c ./deploy/secrets/... ./...":                         true,
		"golangci-lint linters ./deploy/secrets/...":                              true,
		"go vet ./deploy/secrets/...":                                             false,
		"go test -race ./internal/secrets/...":                                    false,
		"go test -count 1 ./internal/secrets/...":                                 false,
		"go test -run TestX ./internal/secrets/... -v":                            false,
		"go test -test.v ./internal/secrets/...":                                  false,
		"go -C sub test ./internal/secrets/...":                                   false,
		"go -C=sub test ./internal/secrets/...":                                   false,
		"go run ./internal/secrets/...":                                           false,
		"go test -coverpkg=./internal/secrets/... ./internal/secrets/...":         false,
		"go test -coverpkg ./internal/secrets/...,./internal/x/... ./...":         false,
		"GOFLAGS=-mod=mod go test ./internal/secrets/...":                         false,
		"golangci-lint run --fix ./internal/secrets/...":                          false,
		"golangci-lint run --new-from-rev HEAD ./internal/secrets/...":            false,
		"staticcheck -tests ./internal/secrets/...":                               false,
		"go vet -printf.funcs=wrapf ./internal/secrets/...":                       false,
		"go test ./internal/secrets/... -coverpkg=./internal/secrets/...":         false,
		"go test ./internal/secrets/... > out.txt":                                false,
		"go test ./internal/secrets/vault":                                        true,
		"go build ./internal/secrets/... && go run ./x.go < ./deploy/secrets/...": true,
	})
}

// TestSecretTokensReadPackagesAfterTheFlagTerminator reads the operands
// after -- the way each program does: the go tool's flag parser, go vet
// and the linters keep reading packages after it, go run its package and
// then the program's arguments, while go test hands it and every word
// after it to the test binary.
func TestSecretTokensReadPackagesAfterTheFlagTerminator(t *testing.T) {
	t.Parallel()
	denies(t, shell.SecretTokens(), "secret-token", map[string]bool{
		"go build -- ./internal/secrets/...":               false,
		"go vet -- ./internal/secrets/...":                 false,
		"go list -- ./internal/secrets/...":                false,
		"go install -- ./internal/secrets/...":             false,
		"go build -race -- ./internal/secrets/... ./x/...": false,
		"go run -- ./internal/secrets/...":                 false,
		"golangci-lint run -- ./internal/secrets/...":      false,
		"staticcheck -- ./internal/secrets/...":            false,
		"go test -- ./internal/secrets/...":                true,
		"go test -race -- ./internal/secrets/...":          true,
		"go run -- ./x.go ./deploy/secrets/...":            true,
		"go build -- ./deploy/secrets/main.txt":            true,
		"go build -- ./... > ./deploy/secrets/...":         true,
	})
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
