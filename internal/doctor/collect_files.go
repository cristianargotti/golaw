package doctor

import (
	"os"
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/install"
)

func collectFiles(input Input) Snapshot {
	snapshot := Snapshot{Files: map[string]string{}}
	receipt, err := install.ReadReceipt(input.ReceiptPath)
	if err != nil {
		return snapshot
	}
	snapshot.Receipt = receipt
	for _, artifact := range receipt.Artifacts {
		data, err := os.ReadFile(artifact.Path)
		if err == nil {
			snapshot.Files[artifact.Path] = string(data)
		}
	}
	snapshot.Settings = readText(receipt.SettingsPath)
	snapshot.Hooks = readText(filepath.Join(receipt.Bundle, "agents", agentClaude, "hooks", "hooks.json"))
	snapshot.Plugin = readText(filepath.Join(receipt.Bundle, "agents", agentClaude, ".claude-plugin", "plugin.json"))
	return snapshot
}
