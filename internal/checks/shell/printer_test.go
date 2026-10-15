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
