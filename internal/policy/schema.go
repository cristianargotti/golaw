package policy

// Schema is the only golaw.yml schema this kit reads.
const Schema = 1

// FileName is the policy file at the repository root.
const FileName = "golaw.yml"

// File is golaw.yml decoded, before validation and compilation. Optional
// scalars are pointers so that an absent key and a zero value stay apart,
// which config explain reports and the tightening check relies on.
type File struct {
	Schema       int             `yaml:"golaw" json:"golaw"`
	Kit          Kit             `yaml:"kit" json:"kit"`
	Profile      *ProfileRef     `yaml:"profile" json:"profile"`
	Language     *string         `yaml:"language" json:"language"`
	Capabilities []string        `yaml:"capabilities" json:"capabilities"`
	Layout       *Layout         `yaml:"layout" json:"layout"`
	Checks       Checks          `yaml:"checks" json:"checks"`
	Ladder       *Ladder         `yaml:"ladder" json:"ladder"`
	Waivers      []Waiver        `yaml:"vulnerability_waivers" json:"vulnerability_waivers"`
	Exceptions   []ExceptionRule `yaml:"exceptions" json:"exceptions"`
	Managed      []ManagedFile   `yaml:"managed" json:"managed"`
}

// Kit pins the exact kit version that governs the repository (A10).
type Kit struct {
	Version string `yaml:"version" json:"version"`
}

// ProfileRef names the pinned profile bundle a repository enrolled (A19).
type ProfileRef struct {
	Name    string `yaml:"name" json:"name"`
	Source  string `yaml:"source" json:"source"`
	Version string `yaml:"version" json:"version"`
	Digest  string `yaml:"digest" json:"digest"`
}

// Layout says where the main package and the decision records live.
type Layout struct {
	Main      *string `yaml:"main" json:"main"`
	Decisions *string `yaml:"decisions" json:"decisions"`
}

// Checks holds the named floor blocks and, inline, the options of single
// checks keyed by check id.
type Checks struct {
	Sizes    *Sizes                    `yaml:"sizes" json:"sizes"`
	Coverage *Coverage                 `yaml:"coverage" json:"coverage"`
	Terms    *Terms                    `yaml:"terms" json:"terms"`
	Lint     *Lint                     `yaml:"lint" json:"lint"`
	Shell    *Shell                    `yaml:"shell" json:"shell"`
	Options  map[string]map[string]any `yaml:",inline" json:"options"`
}

// Sizes are the size floors; each may only fall below the core default.
type Sizes struct {
	FileLines     *int `yaml:"file_lines" json:"file_lines"`
	FunctionLines *int `yaml:"function_lines" json:"function_lines"`
}

// Coverage holds the coverage floors; the percentages may only rise, the
// statement minimum may only fall and the exemption list may only shrink.
type Coverage struct {
	Total          *int     `yaml:"total" json:"total"`
	Package        *int     `yaml:"package" json:"package"`
	File           *int     `yaml:"file" json:"file"`
	MinStatements  *int     `yaml:"min_statements" json:"min_statements"`
	FileExemptions []string `yaml:"file_exemptions" json:"file_exemptions"`
}

// Terms is the configurable tool-name list and its allowed path prefixes.
type Terms struct {
	Tools      []string `yaml:"tools" json:"tools"`
	AllowPaths []string `yaml:"allow_paths" json:"allow_paths"`
}

// Lint names the linter exclusions a repository may keep (M3 reads it).
type Lint struct {
	AllowedExclusions []Exclusion `yaml:"allowed_exclusions" json:"allowed_exclusions"`
}

// Exclusion is one allowed linter exclusion: a path pattern and linters.
type Exclusion struct {
	Path    string   `yaml:"path" json:"path"`
	Linters []string `yaml:"linters" json:"linters"`
}

// Shell holds the optional shell denials of A7.
type Shell struct {
	DenyPush *bool `yaml:"deny_push" json:"deny_push"`
	DenyPR   *bool `yaml:"deny_pr" json:"deny_pr"`
}

// Ladder says which optional local rungs this checkout arms; it is local
// convenience, never a floor (decision 10).
type Ladder struct {
	PreCommit  *bool `yaml:"pre_commit" json:"pre_commit"`
	CommitMsg  *bool `yaml:"commit_msg" json:"commit_msg"`
	PrePush    *bool `yaml:"pre_push" json:"pre_push"`
	TestStaged *bool `yaml:"test_staged" json:"test_staged"`
}

// Waiver is one dated vulnerability waiver of A6.
type Waiver struct {
	ID     string `yaml:"id" json:"id"`
	Until  string `yaml:"until" json:"until"`
	Record string `yaml:"record" json:"record"`
}

// ExceptionRule is one committed exception: a check, an exact path, the
// decision record that justifies it and the day it expires.
type ExceptionRule struct {
	Check  string `yaml:"check" json:"check"`
	Path   string `yaml:"path" json:"path"`
	Record string `yaml:"record" json:"record"`
	Until  string `yaml:"until" json:"until"`
}

// ManagedFile is one file init owns, with the hash it wrote.
type ManagedFile struct {
	Path   string `yaml:"path" json:"path"`
	SHA256 string `yaml:"sha256" json:"sha256"`
}
