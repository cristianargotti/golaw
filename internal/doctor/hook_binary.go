package doctor

import (
	"fmt"
	"os"

	"github.com/cristianargotti/golaw/internal/install"
)

// The line of the binary a Codex or Cursor hook command names, against the
// binary running doctor (section 5.6); its codes are those of M1.
const (
	idBinary             = "doctor.binary"
	codeBinaryMatches    = "binary-matches"
	codeBinaryDiffers    = "binary-differs"
	codeBinaryUnresolved = "binary-unresolved"
	goosWindows          = "windows"
)

// binaryFact is the binary one installed handler names: decoded from the
// quoted path or the encoded launcher, and whether it is the running file.
type binaryFact struct {
	Event string `json:"event"`
	Named string `json:"named"`
	Same  bool   `json:"same"`
	Error string `json:"error"`
}

// collectBinaries decodes the binary every installed handler names, from
// the command the host runs on the operating system: Codex reads
// commandWindows on Windows, Cursor its one command everywhere (D13).
func collectBinaries(receipt install.Receipt, slots map[string][]slot, agent, running, goos string) []binaryFact {
	out := make([]binaryFact, 0, len(receipt.Entries))
	for _, entry := range receipt.Entries {
		index := locate(slots[entry.Event], entry.Definition)
		if index < 0 {
			continue
		}
		fact := binaryFact{Event: entry.Event}
		command := hostCommand(fieldsOf(slots[entry.Event][index].Raw), agent, goos)
		named, found := install.CommandBinary(command)
		if !found {
			fact.Error = "the command " + quoted(command) + " names no binary the installer wrote"
			out = append(out, fact)
			continue
		}
		fact.Named = named
		same, err := sameFile(named, running)
		if err != nil {
			fact.Error = err.Error()
		}
		fact.Same = same
		out = append(out, fact)
	}
	return out
}

// hostCommand is the command the host runs for a handler.
func hostCommand(fields handlerFields, agent, goos string) string {
	if agent == agentCodex && goos == goosWindows && fields.CommandWindows != "" {
		return fields.CommandWindows
	}
	return fields.Command
}

// sameFile compares two paths as files, so a link the installer never
// resolved still matches the binary it leads to.
func sameFile(named, running string) (bool, error) {
	left, err := os.Stat(named)
	if err != nil {
		return false, fmt.Errorf("the hook names %s: %w", named, err)
	}
	right, err := os.Stat(running)
	if err != nil {
		return false, fmt.Errorf("the running binary %s: %w", running, err)
	}
	return os.SameFile(left, right), nil
}

// hookBinaryLines say whether every installed handler names the binary
// running doctor; nothing installed answers no line.
func hookBinaryLines(f agentFacts) []Line {
	if len(f.Binaries) == 0 {
		return nil
	}
	line := Line{ID: idBinary, Key: f.scopeKey(), State: stateOK, Code: codeBinaryMatches, Detail: f.Binaries[0].Named}
	for _, fact := range f.Binaries {
		switch {
		case fact.Error != "":
			line.State, line.Code, line.Detail = stateRed, codeBinaryUnresolved, fact.Event+": "+fact.Error+"; run "+f.installCommand()
			return []Line{line}
		case !fact.Same:
			line.State, line.Code = stateRed, codeBinaryDiffers
			line.Detail = fact.Event + " runs " + fact.Named + ", not the binary running doctor; run " + f.installCommand() + " with the binary the hook should run"
		}
	}
	return []Line{line}
}
