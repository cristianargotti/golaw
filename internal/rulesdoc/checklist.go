package rulesdoc

import (
	"fmt"
	"strings"
)

// Section is one heading of the checklists page with the lines under it,
// its subsections included, up to the next heading of its level or above.
type Section struct {
	Name  string
	Lines []string
}

// heading returns the level and title of a level two or three heading;
// the title heading of the page and deeper levels name no section.
func heading(line string) (int, string, bool) {
	for level := 2; level <= 3; level++ {
		if title, found := strings.CutPrefix(line, strings.Repeat("#", level)+" "); found {
			return level, title, true
		}
	}
	return 0, "", false
}

// Slug names a heading the way its page anchor does: lower case, words
// joined by hyphens, punctuation dropped.
func Slug(title string) string {
	var words []string
	for word := range strings.FieldsSeq(strings.ToLower(title)) {
		words = append(words, strings.Map(keepSlugRune, word))
	}
	return strings.Join(words, "-")
}

// keepSlugRune keeps letters, digits and hyphens and drops the rest.
func keepSlugRune(r rune) rune {
	if r == '-' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
		return r
	}
	return -1
}

// ParseSections returns every level two and three section in page order.
func ParseSections(data []byte) []Section {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var sections []Section
	for i, line := range lines {
		level, title, ok := heading(line)
		if !ok {
			continue
		}
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if next, _, found := heading(lines[j]); found && next <= level {
				end = j
				break
			}
		}
		body := strings.TrimRight(strings.Join(lines[i:end], "\n"), "\n")
		sections = append(sections, Section{Name: Slug(title), Lines: strings.Split(body, "\n")})
	}
	return sections
}

// Find returns the section named exactly, or the only one the name
// starts; an unknown or ambiguous name lists every name.
func Find(sections []Section, name string) (Section, error) {
	var matches []Section
	for _, section := range sections {
		if section.Name == name {
			return section, nil
		}
		if strings.HasPrefix(section.Name, name) {
			matches = append(matches, section)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	names := make([]string, 0, len(sections))
	for _, section := range sections {
		names = append(names, section.Name)
	}
	return Section{}, fmt.Errorf("%w %q; the names are %s", ErrNoChecklist, name, strings.Join(names, ", "))
}
