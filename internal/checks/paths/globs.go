package paths

// SecretGlobs is the name list of section 8.4 in the glob form an agent's
// read permission takes, so the installer writes its deny rules from the
// same data the secret-material check judges: a project relative glob for
// what a repository may hold and a home relative one for the credential
// stores of the person. A source file named like a credential stays
// readable, as the check leaves it to gitleaks, and a documented template
// is not listed, because a glob cannot tell it from the real file; for the
// same reason a dotenv or kubeconfig file named for a stage is listed by
// its usual names, since .env.* and kubeconfig.* would also deny
// .env.example and kubeconfig.go.
func SecretGlobs() []string {
	return []string{
		"**/.env", "**/.env.local", "**/*.env", "**/.envrc",
		"**/.env.dev", "**/.env.development", "**/.env.stage", "**/.env.staging",
		"**/.env.prod", "**/.env.production", "**/.env.test",
		"**/secrets/**", "**/secret/**",
		"**/credentials/*.json", "**/keys/*.json",
		"**/*secrets.json", "**/*secrets.yml", "**/*secrets.yaml",
		"**/*credentials*.json", "**/*credentials*.csv",
		"**/kubeconfig", "**/*.kubeconfig", "**/.kube/**", "~/.kube/**",
		"**/kubeconfig.yaml", "**/kubeconfig.yml", "**/kubeconfig.json", "**/kubeconfig.local",
		"**/kubeconfig.dev", "**/kubeconfig.development", "**/kubeconfig.stage", "**/kubeconfig.staging",
		"**/kubeconfig.prod", "**/kubeconfig.production", "**/kubeconfig.test",
		"**/.docker/config.json", "~/.docker/config.json", "**/.vault-token", "~/.vault-token",
		"**/*.tfstate", "**/*.tfstate.backup", "**/*.tfvars", "**/*.tfvars.json",
		"**/*.pem", "**/*.key", "**/*.p12", "**/*.pfx", "**/*.jks", "**/*.kdbx",
		"**/id_rsa", "**/id_dsa", "**/id_ecdsa", "**/id_ed25519",
		"~/.ssh/id_rsa", "~/.ssh/id_dsa", "~/.ssh/id_ecdsa", "~/.ssh/id_ed25519",
		"~/.aws/credentials", "**/.aws/credentials",
		"**/*service-account*.json", "**/*service_account*.json",
		"~/.config/gcloud/*.json", "~/.azure/**",
		"~/.netrc", "**/.netrc", "**/_netrc", "~/.npmrc", "~/.pypirc",
		"~/.bashrc", "~/.bash_profile", "~/.zshrc", "~/.zprofile", "~/.zshenv", "~/.profile",
	}
}
