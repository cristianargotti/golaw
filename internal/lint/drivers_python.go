package lint

import (
	"path/filepath"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/surface"
)

const (
	pythonRuff    = "ruff"
	pythonPyright = "pyright"

	ruffCheckCommand  = "check"
	ruffFormatCommand = "format"
	ruffNoCache       = "--no-cache"
	ruffConfigFlag    = configFlag
	ruffOutputFlag    = "--output-format"
	ruffJSON          = "json"
	ruffCheckOnly     = "--check"
	ruffConciseForm   = "concise"
	ruffStdinName     = "--stdin-filename"
	ruffStdin         = "-"
	pyrightJSON       = "--outputjson"
	pyrightProject    = "--project"

	// The counts of the fixed arguments, for the capacity of the lists.
	ruffBaseArgs    = 4
	pyrightBaseArgs = 3
)

// pythonDrivers answers the Python tools of the lint target. Only ruff
// check runs in the hook; the format check belongs to the strict set and
// pyright starts too slowly for a hook (M6.6.1).
func pythonDrivers() []Driver {
	config := pythonConfig()
	return []Driver{
		{Tool: pythonRuff, Languages: []surface.Language{surface.Python}, Format: FormatRuff, Args: ruffCheckArgs, HookArgs: ruffHookArgs, Config: config},
		{Tool: pythonRuff, Languages: []surface.Language{surface.Python}, Format: FormatRuffFormat, Args: ruffFormatArgs, Config: config},
		{Tool: pythonPyright, Languages: []surface.Language{surface.Python}, Format: FormatPyright, Args: pyrightArgs, Config: config},
	}
}

// ruffBase passes the kit file with --config, which ruff uses for every
// file in place of a repository's own; --isolated is refused beside
// --config, and --no-cache keeps the checkout free of a cache directory.
func ruffBase(command, configDir string, extra ...string) []string {
	args := make([]string, 0, ruffBaseArgs+len(extra))
	args = append(args, command, ruffNoCache, ruffConfigFlag, filepath.Join(configDir, pythonRuffConfig))
	return append(args, extra...)
}

func ruffCheckArgs(in Input) []string {
	if len(in.Files) == 0 {
		return nil
	}
	return append(ruffBase(ruffCheckCommand, in.ConfigDir, ruffOutputFlag, ruffJSON), pythonPaths(in)...)
}

func ruffHookArgs(in HookInput) []string {
	return ruffBase(ruffCheckCommand, in.ConfigDir, ruffOutputFlag, ruffJSON, ruffStdinName, in.Path, ruffStdin)
}

// ruffFormatArgs asks for the concise form, because the default of ruff
// 0.17 is a diff meant for a person, one line per file being what Parse
// reads.
func ruffFormatArgs(in Input) []string {
	if in.Set != model.SurfaceStrict || len(in.Files) == 0 {
		return nil
	}
	return append(ruffBase(ruffFormatCommand, in.ConfigDir, ruffCheckOnly, ruffOutputFlag, ruffConciseForm), pythonPaths(in)...)
}

func pyrightArgs(in Input) []string {
	if len(in.Files) == 0 {
		return nil
	}
	args := make([]string, 0, pyrightBaseArgs+len(in.Files))
	args = append(args, pyrightJSON, pyrightProject, filepath.Join(in.ConfigDir, pythonPyrightConfig))
	return append(args, pythonPaths(in)...)
}

// pythonPaths names the files absolutely, so the result does not depend on
// the working directory the dispatch chooses.
func pythonPaths(in Input) []string {
	paths := make([]string, 0, len(in.Files))
	for _, file := range in.Files {
		paths = append(paths, filepath.Join(in.Root, filepath.FromSlash(file)))
	}
	return paths
}
