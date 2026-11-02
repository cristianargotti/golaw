package policyfiles_test

import (
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/checks/policyfiles"
	"github.com/cristianargotti/golaw/internal/model"
)

// TestGolangciLetsTheApplicationCallTheSupportPackage keeps the rendered
// lint asset clean: every generated use case imports the support package of
// generate.support, so the application rule allowing it is no open layer,
// while the domain and any other module package stay closed.
func TestGolangciLetsTheApplicationCallTheSupportPackage(t *testing.T) {
	t.Parallel()
	domain := []string{"$gostd", "example.test/service/internal/domain"}
	application := []string{"$gostd", "example.test/service/internal/domain", "example.test/service/internal/application"}
	with := func(list []string, extra ...string) []string { return append(slices.Clone(list), extra...) }
	for _, row := range []struct {
		name, support string
		domain, app   []string
		want          []string
	}{
		{name: "near: the application calls the default support package", domain: domain, app: with(application, "example.test/service/internal/gensupport")},
		{name: "near: the application calls a support package the policy moves", support: "pkg/gen/support", domain: domain, app: with(application, "example.test/service/pkg/gen/support")},
		{name: "near: the application calls a package under the support package", domain: domain, app: with(application, "example.test/service/internal/gensupport/porttest")},
		{name: "red: the domain calls the support package", domain: with(domain, "example.test/service/internal/gensupport"), app: application, want: []string{"golangci-layer-open"}},
		{name: "red: the application calls the default package once the policy moves it", support: "pkg/gen/support", domain: domain, app: with(application, "example.test/service/internal/gensupport"), want: []string{"golangci-layer-open"}},
		{name: "red: the application calls a sibling named like the support package", domain: domain, app: with(application, "example.test/service/internal/gensupportx"), want: []string{"golangci-layer-open"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			files := map[string]string{".golangci.yml": allLinters + layerRules("strict", row.domain, row.app), "go.mod": layerModule}
			res := measure(t, policyfiles.Golangci(), fixture{files: files, capabilities: []string{"go", "hexagonal"}, support: row.support})
			if got := codes(res); res.Status.Kind != model.StatusMeasured || (!slices.Equal(got, row.want) && len(got)+len(row.want) != 0) {
				t.Fatalf("status %+v, codes = %v, want %v (%+v)", res.Status, got, row.want, res.Findings)
			}
		})
	}
}
