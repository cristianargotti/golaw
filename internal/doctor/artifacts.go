package doctor

import "github.com/cristianargotti/golaw/internal/install"

// codeSettingsShared is the settings artifact of a Claude project receipt
// that names the shared settings.json of the repository, which carries the
// path of the person's state directory to every teammate who commits it.
const (
	codeSettingsShared   = "settings-shared"
	sharedSettingsDetail = " is the shared settings file; agent install moves golaw's entries to settings.local.json"
)

func artifactLines(snapshot Snapshot) []Line {
	lines := make([]Line, 0, len(snapshot.Receipt.Artifacts))
	for _, artifact := range snapshot.Receipt.Artifacts {
		line := Line{
			ID: idArtifacts, Key: artifact.Path,
			State: stateOK, Code: "artifact-matches", Detail: artifact.Path,
		}
		content, present := snapshot.Files[artifact.Path]
		switch {
		case artifact.Path == snapshot.Receipt.SettingsPath && install.SharedSettings(snapshot.Receipt):
			line.State, line.Code, line.Detail = stateRed, codeSettingsShared, artifact.Path+sharedSettingsDetail
		case !present:
			line.State, line.Code = stateRed, "artifact-missing"
		case drifted(snapshot.Receipt, artifact, content):
			line.State, line.Code = stateRed, "artifact-drift"
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		lines = append(lines, Line{
			ID: idArtifacts, State: stateRed,
			Code: "receipt-artifacts-empty", Detail: "receipt contains no artifacts",
		})
	}
	return lines
}

// drifted judges the settings file by the entries the receipt owns, because
// the person's own edits elsewhere in that file are not drift of the
// installation; every other artifact is owned whole, byte for byte.
func drifted(receipt install.Receipt, artifact install.Artifact, content string) bool {
	if artifact.Path == receipt.SettingsPath {
		return install.VerifySettings([]byte(content), receipt.Ownership) != nil
	}
	return install.Digest([]byte(content)) != artifact.SHA256
}

func binaryLines(snapshot Snapshot) []Line {
	evidence := snapshot.Binary
	line := Line{ID: "doctor.binary", State: stateOK, Code: "binary-matches"}
	switch {
	case evidence.Source != sourceAgent:
		line.State, line.Code = stateUnknown, "agent-environment-unavailable"
	case evidence.Error != "":
		line.State, line.Code, line.Detail = stateRed, "binary-unresolved", evidence.Error
	case evidence.HookPath == "" || evidence.HookPath != evidence.RunningPath:
		line.State, line.Code = stateRed, "binary-differs"
	default:
		line.Detail = evidence.HookPath
	}
	return []Line{line}
}
