package install

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// removeCodexArtifacts plans the removal of the files a Codex receipt owns
// whole and of its marked block. Each is removed only while its bytes are
// the ones the receipt recorded, so an edit the person made is refused and
// kept; one already gone needs nothing. A path the receipt lists that no
// Codex installation writes is refused, because removing it would act on
// evidence golaw did not produce.
func removeCodexArtifacts(plan *Plan, stateDir string) error {
	for _, artifact := range plan.Receipt.Artifacts {
		var err error
		switch {
		case artifact.Path == codexRulesPath(plan.Receipt):
			err = removeCodexRules(plan, artifact)
		case codexSkillPath(artifact.Path):
			err = removeSharedSkill(plan, stateDir, codexAgent, artifact)
		default:
			err = fmt.Errorf("receipt artifact %s is no file a codex installation writes: %w", artifact.Path, ErrSettings)
		}
		if err != nil {
			return err
		}
	}
	for _, block := range plan.Receipt.Blocks {
		if block.Path != codexBlockPath(plan.Receipt) {
			return fmt.Errorf("receipt block %s is in no file a codex installation writes: %w", block.Path, ErrSettings)
		}
		if err := removeCodexBlock(plan, block); err != nil {
			return err
		}
	}
	return nil
}

// codexSkillPath recognizes the shared skill by the folders both hosts read
// it from, because the receipt alone names it, whatever the home or the
// repository root of the install.
func codexSkillPath(path string) bool {
	suffix := "/" + strings.Join([]string{".agents", "skills", kitName, "SKILL.md"}, "/")
	return filepath.IsAbs(path) && strings.HasSuffix(filepath.ToSlash(path), suffix)
}

func removeCodexRules(plan *Plan, artifact Artifact) error {
	current, exists, err := readOptional(artifact.Path)
	if err != nil || !exists {
		return err
	}
	if Digest(current) != artifact.SHA256 {
		return changedArtifact(artifact.Path, ErrChanged)
	}
	return addChange(plan, artifact.Path, nil, true)
}

// removeCodexBlock takes the marked block out of the instruction file
// with the one line feed after its end marker, which the install wrote
// with it, so an install and uninstall cycle leaves the person's bytes as
// they were. The file goes only when the install created it and nothing
// but blank space is left.
func removeCodexBlock(plan *Plan, block Block) error {
	current, exists, err := readOptional(block.Path)
	if err != nil || !exists {
		return err
	}
	start, end, found, err := agentBlockSpan(current)
	if err != nil {
		return fmt.Errorf("codex instructions %s: %w", block.Path, err)
	}
	if !found {
		return nil
	}
	if Digest(current[start:end]) != block.SHA256 {
		return fmt.Errorf("the golaw block of %s changed since the install: restore it or remove it: %w", block.Path, ErrChanged)
	}
	if end < len(current) && current[end] == '\n' {
		end++
	}
	rest := append(slices.Clone(current[:start]), current[end:]...)
	if block.Created && len(bytes.TrimSpace(rest)) == 0 {
		return addChange(plan, block.Path, nil, true)
	}
	return addChange(plan, block.Path, rest, false)
}
