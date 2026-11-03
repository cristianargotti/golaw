package genmap

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

var (
	// createTable opens a table definition up to its column list.
	createTable = regexp.MustCompile(`(?i)\bcreate\s+table\s+(?:if\s+not\s+exists\s+)?([\w."]+)\s*\(`)
	// alterTable is one ALTER TABLE statement with its actions.
	alterTable = regexp.MustCompile(`(?is)\balter\s+table\s+(?:if\s+exists\s+)?(?:only\s+)?([\w."]+)\s+([^;]*)`)
	// addColumn and dropColumn are the actions of an ALTER TABLE the
	// schema follows.
	addColumn  = regexp.MustCompile(`(?is)^add\s+(?:column\s+)?(?:if\s+not\s+exists\s+)?(.*)$`)
	dropColumn = regexp.MustCompile(`(?is)^drop\s+(?:column\s+)?(?:if\s+exists\s+)?([\w"]+)`)
)

// stopsType reports whether a word of a column definition ends its type.
func stopsType(word string) bool {
	switch strings.ToUpper(word) {
	case "NOT", "NULL", "PRIMARY", "DEFAULT", "REFERENCES", "CHECK", "UNIQUE", "GENERATED", "CONSTRAINT", "COLLATE":
		return true
	}
	return false
}

// opensConstraint reports whether the first word of a list item opens a
// table constraint, not a column.
func opensConstraint(word string) bool {
	switch strings.ToUpper(word) {
	case "PRIMARY", "UNIQUE", "CONSTRAINT", "FOREIGN", "CHECK", "EXCLUDE", "LIKE":
		return true
	}
	return false
}

// table is one table of the migrations: where it is created and its
// columns as "name TYPE", with pk for the primary key and null for a
// column that may be null.
type table struct {
	at   string
	cols []string
}

// readTables replays the up part of every migration in file order: each
// CREATE TABLE, then the columns each ALTER TABLE adds or drops.
func readTables(s model.Snapshot) map[string]*table {
	out := map[string]*table{}
	for _, f := range model.Select(s, func(p string) bool {
		return strings.HasPrefix(p, migrationsDir) && path.Ext(p) == sqlExt
	}) {
		data, err := s.Read(f.Path)
		if err != nil {
			continue
		}
		text := string(data)
		if loc := downMark.FindStringIndex(text); loc != nil {
			text = text[:loc[0]]
		}
		text = lineComment.ReplaceAllStringFunc(text, func(c string) string { return strings.Repeat(" ", len(c)) })
		createTables(out, f.Path, text)
		alterTables(out, text)
	}
	return out
}

// createTables adds every table a migration creates.
func createTables(out map[string]*table, file, text string) {
	for _, loc := range createTable.FindAllStringSubmatchIndex(text, -1) {
		body, ok := parenBody(text, loc[1])
		if !ok {
			continue
		}
		t := &table{at: file + ":" + strconv.Itoa(strings.Count(text[:loc[0]], "\n")+1)}
		for _, item := range topLevelSplit(body) {
			if col, isCol := columnOf(item); isCol {
				t.cols = append(t.cols, col)
			}
		}
		out[tableName(text[loc[2]:loc[3]])] = t
	}
}

// alterTables applies the columns each ALTER TABLE adds or drops.
func alterTables(out map[string]*table, text string) {
	for _, m := range alterTable.FindAllStringSubmatch(text, -1) {
		t, ok := out[tableName(m[1])]
		if !ok {
			continue
		}
		for _, action := range topLevelSplit(m[2]) {
			if add := addColumn.FindStringSubmatch(action); add != nil {
				if col, isCol := columnOf(add[1]); isCol {
					t.cols = append(t.cols, col)
				}
				continue
			}
			if drop := dropColumn.FindStringSubmatch(action); drop != nil {
				name := strings.ToLower(strings.Trim(drop[1], `"`))
				t.cols = slices.DeleteFunc(t.cols, func(c string) bool { return strings.HasPrefix(c, name+" ") })
			}
		}
	}
}

// parenBody answers the text from just after an opening parenthesis to its
// matching close.
func parenBody(text string, open int) (string, bool) {
	depth := 1
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return text[open:i], true
			}
		}
	}
	return "", false
}

// topLevelSplit splits a list on the commas outside parentheses.
func topLevelSplit(body string) []string {
	var out []string
	depth, start := 0, 0
	for i, ch := range body {
		switch {
		case ch == '(':
			depth++
		case ch == ')':
			depth--
		case ch == ',' && depth == 0:
			out = append(out, strings.TrimSpace(body[start:i]))
			start = i + 1
		}
	}
	if rest := strings.TrimSpace(body[start:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

// columnOf reads one column definition: "id TEXT PRIMARY KEY" is "id TEXT
// pk", a column without NOT NULL is marked null; a table constraint is no
// column.
func columnOf(item string) (string, bool) {
	words := strings.Fields(item)
	if len(words) < 2 || opensConstraint(words[0]) {
		return "", false
	}
	typ := []string{}
	for _, w := range words[1:] {
		if stopsType(w) {
			break
		}
		typ = append(typ, w)
	}
	upper := strings.ToUpper(item)
	col := strings.ToLower(strings.Trim(words[0], `"`)) + " " + strings.Join(typ, " ")
	switch {
	case strings.Contains(upper, "PRIMARY KEY"):
		col += " pk"
	case !strings.Contains(upper, "NOT NULL"):
		col += " null"
	}
	return col, true
}
