package doctor

// GenerateTool is one tool golaw setup --gen builds, as the binding found
// it on this machine; State is the resolver's state, verbatim.
type GenerateTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	State   string `json:"state"`
	Path    string `json:"path"`
	Detail  string `json:"detail"`
}

// GenerateFacts is what doctor needs to say which generator binary the
// post hook can run here.
type GenerateFacts struct {
	Tools []GenerateTool `json:"tools"`
}

const (
	idGenTools       = "doctor.gen-tools"
	codeGenAvailable = "gen-tool-available"
	codeGenMissing   = "gen-tool-missing"
	codeGenDrift     = "gen-tool-drift"
	genFix           = "run golaw setup --gen"
)

// AssessGenerate answers one line per tool. No line is ever red: a missing
// binary only means the hook skips that step, while golaw gen and the
// pipeline still run the generator (M6.5.3, M7.11.2).
func AssessGenerate(facts GenerateFacts) []Line {
	lines := make([]Line, 0, len(facts.Tools))
	for _, tool := range facts.Tools {
		lines = append(lines, genLine(tool))
	}
	return lines
}

// genLine reads every state the resolver does not name as usable or drift
// as missing, so a state a later resolver adds still names the fix.
func genLine(tool GenerateTool) Line {
	line := Line{ID: idGenTools, Key: tool.Name, State: stateOK, Code: codeGenAvailable}
	switch tool.State {
	case surfaceVerified:
		line.Detail = tool.Version + " " + tool.Path + " (sha256 verified)"
	case "receipt":
		line.Detail = tool.Version + " " + tool.Path + " (receipt matches)"
	case "path":
		line.Detail = tool.Version + " " + tool.Path + " (on PATH, version matches, checksum not verified)"
	case stateDrift:
		line.State, line.Code = stateDrift, codeGenDrift
		line.Detail = tool.Version + " at " + tool.Path + " no longer matches its receipt: " + genFix
	default:
		line.State, line.Code = setupMissing, codeGenMissing
		line.Detail = tool.Version + ": not built on this machine: " + genFix + "; golaw gen and the pipeline still run it"
	}
	return line
}
