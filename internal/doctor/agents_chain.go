package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// The line of the AGENTS.md budget of Codex (A12): the file Codex reads
// first of the chain of every session in the repository, against the bytes
// Codex reads of the chain, 32 KiB unless its configuration sets another.
const (
	idAgentsChain   = "doctor.agents-chain"
	codeChainWithin = "agents-chain-within"
	codeChainOver   = "agents-chain-over"
	codeChainUnread = "agents-chain-unread"
	chainBudget     = 32 * 1024
)

// chainFiles are the names Codex looks for in a directory of the chain, in
// its order: the first that is a file is the one it reads there.
func chainFiles() []string { return []string{"AGENTS.override.md", "AGENTS.md"} }

// chainFact is the file at the root of the repository that starts the
// AGENTS.md chain of every session, with its size, and the bytes Codex
// reads of the chain; Path is empty when the root holds none.
type chainFact struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Budget int64  `json:"budget"`
	Error  string `json:"error"`
}

// collectChain reads the root file of the chain and the budget it is
// measured against.
func collectChain(input Input) chainFact {
	fact := chainFact{Budget: chainBudget}
	if input.Root == "" {
		return fact
	}
	budget, err := chainBudgetOf(input)
	if err != nil {
		fact.Error = err.Error()
		return fact
	}
	fact.Budget = budget
	for _, name := range chainFiles() {
		path := filepath.Join(input.Root, name)
		info, statErr := os.Stat(path)
		switch {
		case errors.Is(statErr, fs.ErrNotExist):
			continue
		case statErr != nil:
			fact.Error = statErr.Error()
			return fact
		case !info.Mode().IsRegular():
			continue
		}
		fact.Path, fact.Size = path, info.Size()
		return fact
	}
	return fact
}

// chainBudgetOf is the default budget, or the largest project_doc_max_bytes
// the Codex configuration of the person or of the project sets: which layer
// wins depends on how Codex starts, so the largest keeps a budget doctor
// cannot pin from ever making the line red.
func chainBudgetOf(input Input) (int64, error) {
	paths := []string{filepath.Join(input.Root, ".codex", codexConfigName)}
	if input.ConfigDir != "" {
		paths = append(paths, filepath.Join(input.ConfigDir, codexConfigName))
	}
	budget := int64(-1)
	for _, path := range paths {
		config, err := readCodexConfig(path)
		if err != nil {
			return chainBudget, err
		}
		if config.ProjectDocMaxBytes != nil {
			budget = max(budget, *config.ProjectDocMaxBytes)
		}
	}
	if budget < 0 {
		return chainBudget, nil
	}
	return budget, nil
}

// agentsChainLines say, for an installed Codex, whether the root file of
// the AGENTS.md chain fits the bytes Codex reads of the chain: a root file
// over the budget is certain for every session, because every chain starts
// there; the files below the root differ per session and are not measured.
func agentsChainLines(f agentFacts) []Line {
	if f.Agent != agentCodex || f.Receipt.State != receiptRead {
		return nil
	}
	chain := f.Chain
	line := Line{ID: idAgentsChain, Key: f.scopeKey(), State: stateOK, Code: codeChainWithin}
	switch {
	case chain.Error != "":
		line.State, line.Code = stateUnknown, codeChainUnread
		line.Detail = "the root file of the AGENTS.md chain cannot be measured: " + chain.Error
	case chain.Path == "":
		line.Detail = "no AGENTS.md at the repository root; the files below the root, read per session, are not measured"
	case chain.Size > chain.Budget:
		line.State, line.Code = stateRed, codeChainOver
		line.Detail = fmt.Sprintf("%s holds %d bytes and Codex reads %d of the AGENTS.md chain, so it drops the rest, "+
			"the golaw block included when install appended it there: shorten the file to at most %d bytes", chain.Path, chain.Size, chain.Budget, chain.Budget)
	default:
		line.Detail = fmt.Sprintf("%s holds %d of the %d bytes Codex reads of the AGENTS.md chain; "+
			"the files below the root, read per session, are not measured", chain.Path, chain.Size, chain.Budget)
	}
	return []Line{line}
}
