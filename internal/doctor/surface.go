package doctor

import (
	"strings"
)

// SurfaceTool is one pinned surface linter or runtime as the binding found
// it on this machine; State is the resolver's state, verbatim.
type SurfaceTool struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Languages []string `json:"languages"`
	State     string   `json:"state"`
	Path      string   `json:"path"`
	Detail    string   `json:"detail"`
}

// SurfaceFacts is what doctor needs to say what each surface tool can
// judge here: the languages the repository holds, the analyzer scope in
// force, a shape outside M6, and the tools and runtimes those languages
// need.
type SurfaceFacts struct {
	Languages []string      `json:"languages"`
	Scope     []string      `json:"scope"`
	Shape     string        `json:"shape"`
	Tools     []SurfaceTool `json:"tools"`
	Runtimes  []SurfaceTool `json:"runtimes"`
}

const (
	idSurfaceLanguages = "doctor.surface-languages"
	idSurfaceScope     = "doctor.surface-scope"
	idSurfaceShape     = "doctor.surface-shape"
	idSurfaceTools     = "doctor.surface-tools"
	idSurfaceRuntimes  = "doctor.surface-runtimes"

	codeSurfaceLanguages   = "surface-languages"
	codeSurfaceScope       = "surface-scope"
	codeSurfaceShape       = "surface-shape"
	codeToolAvailable      = "surface-tool-available"
	codeToolMissing        = "surface-tool-missing"
	codeToolDrift          = "surface-tool-drift"
	codeRuntimeAvailable   = "surface-runtime-available"
	codeRuntimeMissing     = "surface-runtime-missing"
	stateOutside           = "outside"
	stateDrift             = "drift"
	surfaceFix             = "run golaw setup --surface"
	surfaceVerified        = "verified"
	languagesNone          = "none"
	htmlLanguage           = "html"
	htmlScripts            = "html scripts"
	shapeOutsideSuffix     = ": golaw lints the non-Go surface of a Go repository; the pipeline judges this project"
	surfaceNotLintedPrefix = " not linted on this machine; " + surfaceFix + "; the pipeline judges "
)

// AssessSurface answers the surface lines of doctor. No line is ever red:
// a missing tool only means the pipeline judges what this machine cannot,
// so it is reported, never failed (M6.5.3).
func AssessSurface(facts SurfaceFacts) []Line {
	languages := languagesNone
	if len(facts.Languages) > 0 {
		languages = strings.Join(facts.Languages, ", ")
	}
	lines := []Line{{ID: idSurfaceLanguages, State: stateOK, Code: codeSurfaceLanguages, Detail: languages}}
	if len(facts.Scope) > 0 {
		lines = append(lines, Line{ID: idSurfaceScope, State: stateOK, Code: codeSurfaceScope, Detail: strings.Join(facts.Scope, "; ")})
	}
	if facts.Shape != "" {
		lines = append(lines, Line{ID: idSurfaceShape, State: stateOutside, Code: codeSurfaceShape, Detail: facts.Shape + shapeOutsideSuffix})
	}
	for _, tool := range facts.Tools {
		lines = append(lines, toolLine(tool))
	}
	for _, rt := range facts.Runtimes {
		lines = append(lines, runtimeLine(rt))
	}
	return lines
}

// toolLine reads every state the resolver does not name as missing, so a
// state added later still points at the one fix that works.
func toolLine(tool SurfaceTool) Line {
	line := Line{ID: idSurfaceTools, Key: tool.Name, State: stateOK, Code: codeToolAvailable}
	switch tool.State {
	case surfaceVerified:
		line.Detail = tool.Version + " " + tool.Path + " (sha256 verified)"
	case "receipt":
		line.Detail = tool.Version + " " + tool.Path + " (receipt matches)"
	case "path":
		line.Detail = tool.Version + " " + tool.Path + " (on PATH, version matches, checksum not verified)"
	case stateDrift:
		line.State, line.Code = stateDrift, codeToolDrift
		line.Detail = tool.Version + " at " + tool.Path + " no longer matches its receipt: " + surfaceFix
	default:
		line.State, line.Code = setupMissing, codeToolMissing
		line.Detail = tool.Version + ": " + notLinted(tool.Languages)
	}
	return line
}

func runtimeLine(rt SurfaceTool) Line {
	if rt.State == surfaceVerified {
		return Line{ID: idSurfaceRuntimes, Key: rt.Name, State: stateOK, Code: codeRuntimeAvailable, Detail: rt.Version + " " + rt.Path}
	}
	return Line{ID: idSurfaceRuntimes, Key: rt.Name, State: setupMissing, Code: codeRuntimeMissing, Detail: rt.Detail}
}

// notLinted agrees the verb and the pronoun with the list, and reads html
// as its scripts, the part of an html file a surface tool judges.
func notLinted(languages []string) string {
	verb, pronoun := "is", "it"
	if len(languages) > 1 || (len(languages) == 1 && languages[0] == htmlLanguage) {
		verb, pronoun = "are", "them"
	}
	return languageList(languages) + " " + verb + surfaceNotLintedPrefix + pronoun
}

// languageList joins the languages as a sentence does: a, b and c.
func languageList(languages []string) string {
	names := make([]string, len(languages))
	for i, name := range languages {
		names[i] = name
		if name == htmlLanguage {
			names[i] = htmlScripts
		}
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}
