// The helpers every policy test shares: the example policy documents and
// a strict decode that fails the test.
package policy_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
)

// fullPolicy is the example of specification section 5.3 with every block
// set, as a repository with a profile would commit it.
const fullPolicy = `golaw: 1
kit: {version: 0.1.0}
profile: {name: acme, source: https://example.com/acme.git, version: 0.3.0, digest: "sha256:` + acmeDigest + `"}
language: en-US
capabilities: [go, cli]
layout: {main: root, decisions: docs/adr}
checks:
  sizes: {file_lines: 250, function_lines: 30}
  coverage: {total: 85, package: 80, file: 80, min_statements: 10, file_exemptions: [internal/legacy/old.go]}
  terms: {tools: [acme-tool, other], allow_paths: [agents/, docs/]}
  lint: {allowed_exclusions: [{path: _test\.go, linters: [funlen, mnd]}]}
  shell: {deny_push: true, deny_pr: false}
  text.tool-terms: {strict: true, limit: 3, names: [a, b]}
ladder: {pre_commit: true, commit_msg: true, pre_push: false, test_staged: false}
vulnerability_waivers:
  - {id: GO-2026-6603, until: 2026-11-01, record: docs/adr/0007-go-1-27-2.md}
exceptions:
  - {check: goast.secret-fields, path: internal/auth/session.go, record: docs/adr/0009-session-token.md, until: 2026-12-31}
  - {check: text.dashes, path: docs/old.md, record: docs/adr/0010-old.md, until: 2020-01-01}
managed:
  - {path: .golangci.yml, sha256: ` + abHash + `}
`

// acmeDigest is the digest of the example profile bundle.
const acmeDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// abHash is a well formed sha256 the managed entry carries.
const abHash = "abababababababababababababababababababababababababababababababab"

// minimalPolicy is the smallest valid file.
const minimalPolicy = "golaw: 1\nkit: {version: 0.1.0}\n"

// decode parses a document and fails the test on an error.
func decode(t *testing.T, text string) policy.File {
	t.Helper()
	f, err := policy.Decode([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
