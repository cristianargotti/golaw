package shell_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/shell"
)

func TestCredentialPrint(t *testing.T) {
	t.Parallel()
	denies(t, shell.CredentialPrint(), "credential-print", map[string]bool{
		"gh auth token":                           true,
		"printenv":                                true,
		"env":                                     true,
		"env | grep -i token":                     true,
		"printenv AWS_SECRET_ACCESS_KEY":          true,
		"echo $GITHUB_TOKEN":                      true,
		`echo "${API_KEY}"`:                       true,
		"security find-generic-password -s x -w":  true,
		"aws configure get aws_secret_access_key": true,
		"op read op://vault/item/password":        true,
		"vault read secret/x":                     true,
		"vault kv get kv/x":                       true,
		"kubectl get secret db -o yaml":           true,
		"sudo printenv":                           true,
		"gh auth status":                          false,
		"env GOFLAGS=-mod=mod go build ./...":     false,
		"printenv PATH":                           false,
		"echo $HOME":                              false,
		"kubectl get secrets":                     false,
		"security find-generic-password -s x":     false,
		"go env":                                  false,
	})
}

func TestCredentialPrintReadsEveryArgument(t *testing.T) {
	t.Parallel()
	denies(t, shell.CredentialPrint(), "credential-print", map[string]bool{
		"printenv AWS_SECRET_ACCESS_KEY PATH":       true,
		"printenv -0 NPM_TOKEN":                     true,
		"echo $GITHUB_TOKEN done":                   true,
		"echo TOKEN=$GITHUB_TOKEN":                  true,
		"/usr/bin/printenv":                         true,
		"/usr/bin/printenv GITHUB_TOKEN":            true,
		"printenv.exe GITHUB_TOKEN":                 true,
		`pwsh -Command 'echo $env:GITHUB_TOKEN'`:    true,
		"cmd /c echo %GITHUB_TOKEN%":                true,
		"/usr/bin/printenv PATH":                    false,
		"echo the token expired; echo GITHUB_TOKEN": false,
		"echo $HOME $PATH":                          false,
		"GITHUB_TOKEN=x":                            false,
	})
}
