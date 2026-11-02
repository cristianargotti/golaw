package adaptersql

import (
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterkit"
)

// converted opens every constraint over an existing query, so the agent
// reads first which generated call the hole makes.
const converted = " with the parameters converted to its types; "

// methodConstraint is the one line a method's hole carries: the query to
// call, what its answer becomes and the action its error is wrapped with, so
// the agent decides the domain logic alone and invents no shape.
func methodConstraint(recv string, m adapterkit.Method, q *genkind.Query, mapperName, sentinel string) string {
	wrap := "wrap the error with gensupport.Prefix(" + m.Action + ")"
	if q == nil {
		return "add the sqlc query " + m.Name + " (-- name: " + m.Name + ") and run golaw gen, then call " +
			recv + ".queries." + m.Name + "; " + wrap
	}
	call := "call " + recv + ".queries." + q.Name + converted
	switch {
	case mapperName != "":
		return call + "answer " + mapperName + " of its row and error; " + wrap
	case affected(m, *q):
		return call + "map 0 affected rows to " + sentinel + "; " + wrap
	}
	return call + "convert its answer to the port's types; " + wrap
}

// affected reports an :execrows query behind a method that answers only an
// error, where no affected row is the port's not found answer.
func affected(m adapterkit.Method, q genkind.Query) bool {
	return len(m.Zeros) == 0 && len(q.Results) == 2 && q.Results[0].Type == "int64" && q.Results[1].Type == "error"
}

// mapperConstraint is the hole of a row mapping function.
func mapperConstraint(m mapper, sentinel string) string {
	return "copy the columns of row into " + m.Domain + "; map sql.ErrNoRows to " + sentinel + "; answer every other error as it is"
}

// seedConstraint is the hole of the integration subject.
func seedConstraint(d data) string {
	return "open the integration database of the repository and answer " + d.Suite + ".Subject{Port: " +
		d.Package + ".New" + d.Type + "(db), Seed: insert the rows each case's Args and Want need}"
}
