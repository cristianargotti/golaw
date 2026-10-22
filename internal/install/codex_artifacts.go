package install

import (
	"fmt"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/assets"
)

const (
	codexBlockAsset = "agents/codex/AGENTS.md"
	codexBlockName  = "AGENTS.md"
	codexRulesDir   = "rules"
	codexRulesName  = "golaw.rules"
)

// addCodexArtifacts plans what a Codex installation owns besides its
// handlers: the rules file with --scope project only, because a user rules
// file would forbid its prefixes in every Codex session of the person,
// in repositories golaw never enrolled too, where the hook allows them;
// the shared skill; and the marked block of the instruction file.
func addCodexArtifacts(plan *Plan, previous Receipt, options Options) error {
	if plan.Receipt.Scope == scopeProject {
		if err := addCodexRules(plan, codexRulesPath(plan.Receipt), previous); err != nil {
			return err
		}
	}
	if err := addSharedSkill(plan, options); err != nil {
		return err
	}
	return addCodexBlock(plan, previous, codexBlockPath(plan.Receipt))
}

// codexRulesPath is derived from the receipt, so uninstall finds the rules
// file without the options of the install that wrote it.
func codexRulesPath(receipt Receipt) string {
	return filepath.Join(filepath.Dir(receipt.HooksPath), codexRulesDir, codexRulesName)
}

// codexBlockPath is the instruction file Codex reads for the scope: the
// one of the Codex home, or the one at the root of the repository, whose
// .codex folder holds the project hooks file.
func codexBlockPath(receipt Receipt) string {
	directory := filepath.Dir(receipt.HooksPath)
	if receipt.Scope == scopeProject {
		directory = filepath.Dir(directory)
	}
	return filepath.Join(directory, codexBlockName)
}

// addCodexBlock merges the marked block into the instruction file and
// records the hash of the block alone, since the rest of the file is the
// person's. A block the receipt recorded that someone edited is refused,
// never overwritten; one removed since is written again. The file counts
// as created when the install wrote it, which uninstall needs to know.
func addCodexBlock(plan *Plan, previous Receipt, path string) error {
	source, err := assets.Read(codexBlockAsset)
	if err != nil {
		return fmt.Errorf("load the codex instructions: %w", err)
	}
	current, exists, err := readOptional(path)
	if err != nil {
		return err
	}
	recorded, known := previousBlock(previous, path)
	start, end, found, err := agentBlockSpan(current)
	if err != nil {
		return fmt.Errorf("codex instructions %s: %w", path, err)
	}
	if known && found && Digest(current[start:end]) != recorded.SHA256 {
		return fmt.Errorf("the golaw block of %s changed since the install: restore it or remove it: %w", path, ErrChanged)
	}
	merged, err := mergeAgentBlock(current, source)
	if err != nil {
		return err
	}
	start, end, _, err = agentBlockSpan(merged)
	if err != nil {
		return err
	}
	plan.Receipt.Blocks = append(plan.Receipt.Blocks, Block{
		Path: path, SHA256: Digest(merged[start:end]), Created: !exists || (known && recorded.Created),
	})
	return addChange(plan, path, merged, false)
}

func previousBlock(receipt Receipt, path string) (Block, bool) {
	for _, block := range receipt.Blocks {
		if block.Path == path {
			return block, true
		}
	}
	return Block{}, false
}
