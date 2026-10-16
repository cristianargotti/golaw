package paths

// SecretGlobs are the Read deny rules agent install writes for the
// reference gap, a file named in a prompt that the host reads without a
// hook: private keys and keystores, the SSH keys and the cloud credential
// stores of the person, which no source tree needs to read. The host
// applies these user settings to every project before any hook runs, and a
// rule cannot exempt a template without lifting the person's own denies, so
// the two families that have templates are named narrowly: the variable
// files Terraform loads by itself and the names that end in
// service-account.json or service_account.json. A template of either stays
// readable in every project, and every other secret name stays with the
// hook, which judges each read by the whole name list of section 8.4.
func SecretGlobs() []string {
	return []string{
		"**/*.pem", "**/*.key", "**/*.p12", "**/*.pfx", "**/*.jks", "**/*.kdbx",
		"**/terraform.tfvars", "**/terraform.tfvars.json", "**/*.auto.tfvars", "**/*.auto.tfvars.json",
		"~/.ssh/id_rsa", "~/.ssh/id_dsa", "~/.ssh/id_ecdsa", "~/.ssh/id_ed25519",
		"~/.aws/credentials", "~/.config/gcloud/*.json", "~/.azure/**",
		"**/*service-account.json", "**/*service_account.json",
	}
}
