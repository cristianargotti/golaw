// Package profile reads a pinned, data-only profile bundle (specification
// section 5.4): the identity stamp the policy package fixed, the digest
// over every other file, and rules.yml decoded strictly into the data the
// generic evaluators of the core read. A bundle is never fetched during a
// hook; the human-run lifecycle of A19 installs it under the config
// directory, and the loader only reads what is there.
package profile
