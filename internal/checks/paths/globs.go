package paths

// SecretGlobs are the narrow Read deny rules of section 8.2 in the glob
// form an agent's read permission takes: private key files, Terraform
// variable files, the SSH keys and the cloud credential stores of the
// person. They close the reference gap, a file named in a prompt that the
// host reads without a hook, and they land in the user settings, which the
// host applies to every project, enrolled or not, before any hook runs; so
// they name only files no source tree needs to read, and the broad name
// list of section 8.4 stays with the hook, which judges every read in its
// repository and leaves a source file named like a credential readable.
func SecretGlobs() []string {
	return []string{
		"**/*.pem", "**/*.key", "**/*.p12", "**/*.pfx", "**/*.jks", "**/*.kdbx",
		"**/*.tfvars", "**/*.tfvars.json",
		"~/.ssh/id_rsa", "~/.ssh/id_dsa", "~/.ssh/id_ecdsa", "~/.ssh/id_ed25519",
		"~/.aws/credentials", "~/.config/gcloud/*.json", "~/.azure/**",
		"**/*service-account*.json", "**/*service_account*.json",
	}
}
