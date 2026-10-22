package main

import "context"

// The m4-codex category: the corpus of the survey and the proof manifest
// of the Codex adapter, replayed through the installed Codex command.
const (
	m4CodexCategory = "m4-codex"
	m4CodexCorpus   = "internal/hook/codex/testdata/payloads"
	m4CodexManifest = "internal/hook/codex/proof/manifest.yml"
)

// m4CodexCases are the eight payloads of the survey (D19) with the
// repository each needs to answer as a session would: the patches apply
// exactly to the files they name, the post events report clean files, so
// every payload is allowed and a denial is a false one.
func m4CodexCases() map[string]m4CorpusCase {
	return map[string]m4CorpusCase{
		"pre-apply-patch-add": {want: m4Allow},
		"pre-apply-patch-update-move-delete": {want: m4Allow, files: map[string]string{
			"internal/app/old.go":  "package app\n\nfunc Old() {\n\treturn\n}\n",
			"internal/app/gone.go": "package app\n",
		}},
		"pre-bash-plain":         {want: m4Allow},
		"pre-bash-spawned":       {want: m4Allow},
		"pre-bash-heredoc-patch": {want: m4Allow},
		"pre-bash-heredoc-patch-cd": {want: m4Allow, files: map[string]string{
			"sub/dir/main.go": "package dir\n\nconst text = `\nold\n`\n",
		}},
		"post-bash":        {want: m4Allow, pre: true, written: map[string]string{"notes/out.txt": "x"}},
		"post-apply-patch": {want: m4Allow, files: map[string]string{"notes/hello.txt": "hello\n"}},
	}
}

// m4CodexRows is the m4-codex category.
func (s *suite) m4CodexRows(ctx context.Context, _ bool) []row {
	return s.m4AdapterRows(ctx, m4Adapter{
		agent: agentCodex, category: m4CodexCategory, corpus: m4CodexCorpus,
		manifest: m4CodexManifest, cases: m4CodexCases(),
	})
}
