// Package kinds is the registry of the generator kinds phase M7.1 ships: a
// kind joins by its own package and one line here, so the commands never
// name a kind package themselves.
package kinds

import (
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adapterhttp"
	"github.com/cristianargotti/golaw/internal/gen/kinds/adaptersql"
	"github.com/cristianargotti/golaw/internal/gen/kinds/handler"
	"github.com/cristianargotti/golaw/internal/gen/kinds/port"
	"github.com/cristianargotti/golaw/internal/gen/kinds/support"
	"github.com/cristianargotti/golaw/internal/gen/kinds/usecase"
	"github.com/cristianargotti/golaw/internal/gen/kinds/wiring"
)

// All answers every registered kind in name order. The slice is new on each
// call, so no caller can reorder or edit the registry another caller reads.
func All() []genkind.Kind {
	return []genkind.Kind{
		adapterhttp.Kind(), adaptersql.Kind(), handler.Kind(), port.Kind(),
		support.Kind(), usecase.Kind(), wiring.Kind(),
	}
}

// Lookup answers the registered kind of that name.
func Lookup(name string) (genkind.Kind, bool) {
	for _, k := range All() {
		if k.Name() == name {
			return k, true
		}
	}
	return nil, false
}
