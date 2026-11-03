package genmap

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

// The directories the pinned sqlc reads, fixed by the kit's sqlc.yaml.
const (
	queriesDir    = "db/queries/"
	migrationsDir = "db/migrations/"
)

var (
	// queryName is the sqlc annotation of a query: "-- name: GetPlan :one".
	queryName = regexp.MustCompile(`^--\s*name:\s*(\w+)\s+:(\w+)`)
	// tableUse is a table a statement reads or writes.
	tableUse = regexp.MustCompile(`(?i)\b(?:from|into|update|join)\s+([a-z_"][\w."]*)`)
	// downMark opens the down part of a migration, which the schema skips.
	downMark = regexp.MustCompile(`(?im)^--\s*(?:\+goose\s+down|migrate:down)`)
	// lineComment is an SQL comment to the end of its line.
	lineComment = regexp.MustCompile(`--[^\n]*`)
)

// sqlQuery is one sqlc query: its name, its kind and the tables it names.
type sqlQuery struct {
	name, kind string
	tables     []string
}

// queryFile is one file of the queries directory.
type queryFile struct {
	path    string
	queries []sqlQuery
}

// readQueryFiles parses every file of the queries directory, by path.
func readQueryFiles(s model.Snapshot) []queryFile {
	var out []queryFile
	for _, f := range model.Select(s, func(p string) bool {
		return strings.HasPrefix(p, queriesDir) && path.Ext(p) == sqlExt
	}) {
		data, err := s.Read(f.Path)
		if err == nil {
			out = append(out, queryFile{path: f.Path, queries: parseQueries(string(data))})
		}
	}
	return out
}

// parseQueries reads the annotated queries of one file with the tables of
// the statement each annotation opens.
func parseQueries(text string) []sqlQuery {
	var out []sqlQuery
	for line := range strings.SplitSeq(text, "\n") {
		if m := queryName.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			out = append(out, sqlQuery{name: m[1], kind: m[2]})
			continue
		}
		if len(out) == 0 {
			continue
		}
		q := &out[len(out)-1]
		for _, m := range tableUse.FindAllStringSubmatch(lineComment.ReplaceAllString(line, ""), -1) {
			if name := tableName(m[1]); !slices.Contains(q.tables, name) {
				q.tables = append(q.tables, name)
			}
		}
	}
	return out
}

// tableName drops the quotes and the public schema of a table name.
func tableName(raw string) string {
	name := strings.ToLower(strings.ReplaceAll(raw, `"`, ""))
	return strings.TrimPrefix(name, "public.")
}

// areaQueries answers the query files of the area: the ones its manifest
// entries name as contracts, else the ones named after a word of its
// ports, use cases or types.
func (r *areaReader) areaQueries() []queryFile {
	var named []string
	if m := r.in.Manifest; m != nil {
		for i := range m.Entries {
			for _, c := range m.Entries[i].Contracts {
				if m.Entries[i].Area() == r.in.Area && strings.HasPrefix(c.Path, queriesDir) {
					named = append(named, c.Path)
				}
			}
		}
	}
	var out []queryFile
	for _, qf := range readQueryFiles(r.in.Snapshot) {
		if slices.Contains(named, qf.path) || r.namesWord(strings.TrimSuffix(path.Base(qf.path), sqlExt)) {
			out = append(out, qf)
		}
	}
	return out
}

// sqlDecls answers the columns of every table the area's queries touch,
// in the order the queries name them, and the queries of each file.
func (r *areaReader) sqlDecls(files []queryFile) ([]Decl, []Decl) {
	tables := readTables(r.in.Snapshot)
	var tableDecls []Decl
	queryDecls := make([]Decl, 0, len(files))
	seen := map[string]bool{}
	for _, qf := range files {
		names := make([]string, 0, len(qf.queries))
		for _, q := range qf.queries {
			names = append(names, q.name+" :"+q.kind)
			for _, name := range q.tables {
				t, ok := tables[name]
				if ok && !seen[name] {
					seen[name] = true
					tableDecls = append(tableDecls, Decl{Name: name, At: t.at, Lines: []string{strings.Join(t.cols, ", ")}})
				}
			}
		}
		queryDecls = append(queryDecls, Decl{Name: qf.path, Lines: []string{strings.Join(names, ", ")}})
	}
	return tableDecls, queryDecls
}
