package paths_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/paths"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/proof"
)

// read dispatches the secret material check over one read event.
func read(t *testing.T, path string) model.Result {
	t.Helper()
	event := &model.Event{Shape: model.ShapeRead, Agent: "tester", Tool: "Read", Read: &model.ReadRequest{Path: path}}
	in := model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: proof.NewTree(nil), Event: event}
	res, err := paths.SecretMaterial().Run(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestSecretMaterial(t *testing.T) {
	t.Parallel()
	denied := []string{
		".env", "/work/app/.env.local", ".env.production", "prod.env", ".envrc",
		"secrets/nested/key.yaml", "config/secrets.json", "secrets.yml",
		"credentials/app.json", "keys/signing.json",
		"kubeconfig", "cluster.kubeconfig", ".kube/config",
		".docker/config.json", ".vault-token",
		"terraform.tfstate", "terraform.tfstate.backup", "prod.tfvars", "prod.tfvars.json",
		"deploy.pem", "server.key", "store.p12", "cert.pfx", "trust.jks", "vault.kdbx",
		".ssh/id_rsa", "id_ed25519",
		".aws/credentials", "service-account.json", "sa_service_account.json", ".config/gcloud/adc.json", ".azure/accessTokens.json",
		".netrc", "_netrc", ".npmrc", ".pypirc",
		".zshrc", ".bash_profile", ".profile",
		"aws-credentials.csv", "credentials",
	}
	for _, path := range denied {
		if res := read(t, path); len(res.Findings) != 1 || res.Findings[0].Code != "secret-material" || res.Status.Kind != model.StatusMeasured {
			t.Errorf("%s: %+v", path, res)
		}
	}
	allowed := []string{
		"internal/credentials.go", "secrets/client.go", "docs/secrets.md", "cmd/keys/main.go", "keys.ts",
		".env.example", ".env.sample", "id_rsa.pub", "config/app.yaml", "README.md", "Makefile", "profile.png", "token.go",
	}
	for _, path := range allowed {
		if res := read(t, path); len(res.Findings) != 0 || res.Status.Kind != model.StatusMeasured {
			t.Errorf("%s: %+v", path, res)
		}
	}
}

func TestSecretMaterialJudgesReadsOnly(t *testing.T) {
	t.Parallel()
	event := &model.Event{Shape: model.ShapeShell, Agent: "tester", Tool: "Bash", Shell: &model.ShellCommand{Command: "cat deploy.pem", Cwd: "/work", Tool: "Bash"}}
	in := model.Input{Mode: model.ModeHook, Policy: proof.Policy(), Snapshot: proof.NewTree(nil), Event: event}
	res, err := paths.SecretMaterial().Run(t.Context(), in)
	if err != nil || res.Status.Kind != model.StatusNotApplicable {
		t.Errorf("shell event = %+v, %v", res, err)
	}
}
