package install

import (
	"fmt"
	"regexp"
)

var tagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

type catalog struct {
	Name  string `json:"name"`
	Owner struct {
		Name string `json:"name"`
	} `json:"owner"`
	Plugins []struct {
		Name   string `json:"name"`
		Source struct {
			Source string `json:"source"`
			URL    string `json:"url"`
			Path   string `json:"path"`
			Ref    string `json:"ref"`
		} `json:"source"`
	} `json:"plugins"`
}

// ValidateCatalog prevents distribution from following an unpinned branch.
func ValidateCatalog(data []byte) error {
	var value catalog
	if err := strictJSON(data, &value); err != nil {
		return err
	}
	if value.Name != kitName || len(value.Plugins) != 1 {
		return fmt.Errorf("catalog identity: %w", ErrSettings)
	}
	plugin := value.Plugins[0]
	source := plugin.Source
	if plugin.Name != kitName || source.Source != "git-subdir" ||
		source.URL != "https://github.com/cristianargotti/golaw.git" ||
		source.Path != "agents/claude" || !tagPattern.MatchString(source.Ref) {
		return fmt.Errorf("catalog pin: %w", ErrSettings)
	}
	return nil
}
