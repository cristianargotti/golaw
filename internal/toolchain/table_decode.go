package toolchain

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/surface"
)

// quarantine is the seven day dependency law of spec section 3: a release
// enters the table only once it has been public this long.
const quarantine = 7 * 24 * time.Hour

// toolName keeps a tool name usable as a directory and a receipt file name.
var toolName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// sha256Hex is the only checksum shape the installer compares.
var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

type tableFile struct {
	Read     string         `yaml:"read"`
	Runtimes []runtimeEntry `yaml:"runtimes"`
	Tools    []toolEntry    `yaml:"tools"`
}

type runtimeEntry struct {
	Name  string   `yaml:"name"`
	Floor string   `yaml:"floor"`
	Below string   `yaml:"below"`
	Probe []string `yaml:"probe"`
}

type toolEntry struct {
	Name         string       `yaml:"name"`
	Version      string       `yaml:"version"`
	Published    string       `yaml:"published"`
	Distribution string       `yaml:"distribution"`
	Languages    []string     `yaml:"languages"`
	Hook         bool         `yaml:"hook"`
	Probe        []string     `yaml:"probe"`
	Bin          string       `yaml:"bin"`
	Assets       []assetEntry `yaml:"assets"`
}

type assetEntry struct {
	Platform string `yaml:"platform"`
	URL      string `yaml:"url"`
	SHA256   string `yaml:"sha256"`
	Archive  string `yaml:"archive"`
	Member   string `yaml:"member"`
}

// decodeTable reads the table strictly: a key the kit does not know is a
// pin nobody reviewed, so it is refused rather than ignored.
func decodeTable(data []byte) (tableFile, error) {
	var file tableFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return tableFile{}, fmt.Errorf("%w: %w", ErrTable, err)
	}
	read, err := time.Parse(time.DateOnly, file.Read)
	if err != nil {
		return tableFile{}, fmt.Errorf("%w: read %q: %w", ErrTable, file.Read, err)
	}
	if err := validateRuntimes(file.Runtimes); err != nil {
		return tableFile{}, err
	}
	seen := make(map[string]bool, len(file.Tools))
	for i := range file.Tools {
		entry := &file.Tools[i]
		if seen[entry.Name] {
			return tableFile{}, fmt.Errorf("%w: tool %q repeated", ErrTable, entry.Name)
		}
		seen[entry.Name] = true
		if err := validateTool(entry, read); err != nil {
			return tableFile{}, err
		}
	}
	return file, nil
}

func validateRuntimes(runtimes []runtimeEntry) error {
	names := make([]string, 0, len(runtimes))
	for _, entry := range runtimes {
		names = append(names, entry.Name)
		if !semver.IsValid("v" + entry.Floor) {
			return fmt.Errorf("%w: runtime %s floor %q", ErrTable, entry.Name, entry.Floor)
		}
		if entry.Below != "" && (!semver.IsValid("v"+entry.Below) || semver.Compare("v"+entry.Below, "v"+entry.Floor) <= 0) {
			return fmt.Errorf("%w: runtime %s below %q", ErrTable, entry.Name, entry.Below)
		}
		if len(entry.Probe) == 0 {
			return fmt.Errorf("%w: runtime %s without probe", ErrTable, entry.Name)
		}
	}
	if !slices.Equal(names, []string{runtimeNode, runtimePython}) {
		return fmt.Errorf("%w: runtimes %v, want node then python", ErrTable, names)
	}
	return nil
}

func validateTool(entry *toolEntry, read time.Time) error {
	published, err := time.Parse(time.DateOnly, entry.Published)
	switch {
	case !toolName.MatchString(entry.Name):
		return fmt.Errorf("%w: tool name %q", ErrTable, entry.Name)
	case !semver.IsValid("v" + entry.Version):
		return fmt.Errorf("%w: %s version %q", ErrTable, entry.Name, entry.Version)
	case err != nil:
		return fmt.Errorf("%w: %s published %q: %w", ErrTable, entry.Name, entry.Published, err)
	case published.Add(quarantine).After(read):
		return fmt.Errorf("%w: %s published %s, inside the seven days before %s", ErrTable, entry.Name, entry.Published, read.Format(time.DateOnly))
	case len(entry.Probe) == 0:
		return fmt.Errorf("%w: %s without probe", ErrTable, entry.Name)
	}
	if err := validateLanguages(entry); err != nil {
		return err
	}
	switch Distribution(entry.Distribution) {
	case DistBinary:
		if entry.Bin != "" || len(entry.Assets) == 0 {
			return fmt.Errorf("%w: binary %s needs assets and no bin", ErrTable, entry.Name)
		}
		return validateAssets(entry)
	case DistNPM, DistPyPI:
		if entry.Hook || entry.Bin == "" || len(entry.Assets) != 0 {
			return fmt.Errorf("%w: %s %s needs a bin, no asset and no hook", ErrTable, entry.Distribution, entry.Name)
		}
		return nil
	default:
		return fmt.Errorf("%w: %s distribution %q", ErrTable, entry.Name, entry.Distribution)
	}
}

func validateLanguages(entry *toolEntry) error {
	if len(entry.Languages) == 0 {
		return fmt.Errorf("%w: %s without languages", ErrTable, entry.Name)
	}
	known := surface.Languages()
	for _, language := range entry.Languages {
		if !slices.Contains(known, surface.Language(language)) {
			return fmt.Errorf("%w: %s language %q", ErrTable, entry.Name, language)
		}
	}
	return nil
}

func validateAssets(entry *toolEntry) error {
	platforms := platforms()
	seen := make(map[string]bool, len(entry.Assets))
	for _, asset := range entry.Assets {
		switch {
		case !slices.Contains(platforms, asset.Platform) || seen[asset.Platform]:
			return fmt.Errorf("%w: %s platform %q unknown or repeated", ErrTable, entry.Name, asset.Platform)
		case !sha256Hex.MatchString(asset.SHA256):
			return fmt.Errorf("%w: %s %s sha256 %q", ErrTable, entry.Name, asset.Platform, asset.SHA256)
		case !strings.HasPrefix(asset.URL, "https://"):
			return fmt.Errorf("%w: %s %s url %q is not https", ErrTable, entry.Name, asset.Platform, asset.URL)
		case !slices.Contains([]string{"", archiveTarGz, archiveZip}, asset.Archive):
			return fmt.Errorf("%w: %s %s archive %q", ErrTable, entry.Name, asset.Platform, asset.Archive)
		case (asset.Archive == "") != (asset.Member == ""):
			return fmt.Errorf("%w: %s %s archive %q with member %q", ErrTable, entry.Name, asset.Platform, asset.Archive, asset.Member)
		}
		seen[asset.Platform] = true
	}
	return nil
}

// tools converts the entries in file order.
func (f tableFile) tools() []Tool {
	out := make([]Tool, 0, len(f.Tools))
	for i := range f.Tools {
		entry := &f.Tools[i]
		tool := Tool{
			Name: entry.Name, Version: entry.Version, Distribution: Distribution(entry.Distribution),
			Hook: entry.Hook, Probe: entry.Probe, Bin: entry.Bin,
		}
		for _, language := range entry.Languages {
			tool.Languages = append(tool.Languages, surface.Language(language))
		}
		for _, asset := range entry.Assets {
			tool.Assets = append(tool.Assets, Asset(asset))
		}
		out = append(out, tool)
	}
	return out
}
