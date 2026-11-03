package port

import (
	"fmt"
	"go/token"
	"slices"
	"strings"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
)

// validate refuses a request the layout or the templates cannot place
// before anything is read, so a typo never reaches the contract reader.
func validate(req genkind.Request) error {
	area, typ := req.Name.Area, req.Name.Type
	if !token.IsIdentifier(area) || area != strings.ToLower(area) || !token.IsIdentifier(typ) || !token.IsExported(typ) {
		return fmt.Errorf("%w: %q is not <area>.<Type>: the area a lower case Go name, the type an exported Go name", genkind.ErrRequest, req.Name.String())
	}
	if req.Usecase != "" && (!token.IsIdentifier(req.Usecase) || !token.IsExported(req.Usecase)) {
		return fmt.Errorf("%w: --usecase %s is not an exported Go name", genkind.ErrRequest, req.Usecase)
	}
	return nil
}

// consumer refuses a port without --usecase in an area whose use cases
// golaw generates (A56), because there golaw writes the Deps that consume
// it. In an area whose use cases are written by hand the consumer is the
// person's code, so the port is adopted or stubbed without one.
func consumer(m manifest.Manifest, req genkind.Request) error {
	if req.Usecase != "" || req.Port != (genkind.Name{}) {
		return nil
	}
	for i := range m.Entries {
		if m.Entries[i].Kind == genkind.KindUsecase && m.Entries[i].Area() == req.Name.Area {
			return fmt.Errorf("%w: golaw new port %s needs --usecase: golaw writes the use cases of %s, so a port is created with the use case that consumes it",
				genkind.ErrRequest, req.Name, req.Name.Area)
		}
	}
	return nil
}

// needs asks for the consuming use case (A56) unless the use case asked for
// this port itself or already lists it, so a chain of needs ends after one
// step and no port is left without its consumer. Without --usecase the
// consumer is hand written and nothing is asked.
func needs(m manifest.Manifest, req genkind.Request) []genkind.Need {
	if req.Port != (genkind.Name{}) || req.Usecase == "" {
		return nil
	}
	name := genkind.Name{Area: req.Name.Area, Type: req.Usecase}
	entry, _ := m.Lookup(genkind.KindUsecase, name.String())
	if slices.Contains(entry.Ports, req.Name.Type) {
		return nil
	}
	return []genkind.Need{{Kind: genkind.KindUsecase, Request: genkind.Request{
		Name:  name,
		Ports: append(slices.Clone(entry.Ports), req.Name.Type),
		In:    entry.In,
		Out:   entry.Out,
		Port:  req.Name,
	}}}
}
