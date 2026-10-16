package doctor

import "runtime/debug"

// BuildMetadata records source information without authenticating an executable.
type BuildMetadata struct {
	Version  string `json:"version"`
	Sum      string `json:"sum"`
	Revision string `json:"revision"`
}

// RunningMetadata keeps source builds separate from release verification.
func RunningMetadata() BuildMetadata {
	info, present := debug.ReadBuildInfo()
	if !present {
		return BuildMetadata{}
	}
	return Metadata(info)
}

// Metadata permits source-build states to be tested without linker changes.
func Metadata(info *debug.BuildInfo) BuildMetadata {
	if info == nil {
		return BuildMetadata{}
	}
	out := BuildMetadata{Version: info.Main.Version, Sum: info.Main.Sum}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			out.Revision = setting.Value
		}
	}
	return out
}
