package profile_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/profile"
)

func validData(t *testing.T) model.ProfileData {
	t.Helper()
	d, err := profile.Decode([]byte(fixtureRules))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestValidateAcceptsTheFixture(t *testing.T) {
	t.Parallel()
	if err := profile.Validate(validData(t)); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRefusesEachBrokenField(t *testing.T) {
	t.Parallel()
	equals := "maybe"
	for name, test := range map[string]struct {
		change func(*model.ProfileData)
		want   string
	}{
		"upper case term":      {func(d *model.ProfileData) { d.Terms = []string{"LegacyCI"} }, "terms"},
		"empty term":           {func(d *model.ProfileData) { d.Terms = []string{""} }, "terms"},
		"absolute path":        {func(d *model.ProfileData) { d.Paths = []string{"/etc/notes"} }, "paths"},
		"parent path":          {func(d *model.ProfileData) { d.Paths = []string{"../notes"} }, "paths"},
		"bad glob":             {func(d *model.ProfileData) { d.Paths = []string{"[notes"} }, "paths"},
		"layout main":          {func(d *model.ProfileData) { d.Layout = &model.Layout{Main: "lib", Decisions: "docs/adr"} }, "layout.main"},
		"layout decisions":     {func(d *model.ProfileData) { d.Layout = &model.Layout{Main: "root", Decisions: "/adr"} }, "layout.decisions"},
		"template prefix":      {func(d *model.ProfileData) { d.Templates.Exclude = []string{"../.github/"} }, "templates.exclude"},
		"target":               {func(d *model.ProfileData) { d.Targets = []string{"Hooks Test"} }, "targets"},
		"stage twice":          {func(d *model.ProfileData) { d.Dockerfile.Stages = []string{"base", "base"} }, "dockerfile.stages"},
		"stage case":           {func(d *model.ProfileData) { d.Dockerfile.Stages = []string{"Base"} }, "dockerfile.stages"},
		"install stage":        {func(d *model.ProfileData) { d.Dockerfile.Installs[0].Stage = "" }, "dockerfile.installs"},
		"install package":      {func(d *model.ProfileData) { d.Dockerfile.Installs[0].Packages = []string{"git tzdata"} }, "dockerfile.installs"},
		"ignore comment":       {func(d *model.ProfileData) { d.Dockerfile.IgnoreComments = []string{"lint ignore"} }, "dockerfile.ignore_comments"},
		"registry pattern":     {func(d *model.ProfileData) { d.Registry.Pattern = "(" }, "registry.pattern"},
		"registry empty":       {func(d *model.ProfileData) { d.Registry.Pattern = "" }, "registry.pattern"},
		"assertion path":       {func(d *model.ProfileData) { d.Assertions[0].Path = "" }, "assertions[0].path"},
		"assertion glob":       {func(d *model.ProfileData) { d.Assertions[0].Path = "deploy/[x.yaml" }, "assertions[0].path"},
		"assertion key":        {func(d *model.ProfileData) { d.Assertions[1].Key = nil }, "assertions[1].key"},
		"assertion segment":    {func(d *model.ProfileData) { d.Assertions[1].Key = []string{"a", ""} }, "assertions[1].key"},
		"assertion kind":       {func(d *model.ProfileData) { d.Assertions[2].Kind = "float" }, "assertions[2].kind"},
		"assertion missing":    {func(d *model.ProfileData) { d.Assertions[2].Missing = "skip" }, "assertions[2].missing"},
		"assertion files":      {func(d *model.ProfileData) { d.Assertions[2].Files = "" }, "assertions[2].files"},
		"assertion bool value": {func(d *model.ProfileData) { d.Assertions[0].Equals = &equals }, "assertions[0].equals"},
		"assertion int value":  {func(d *model.ProfileData) { d.Assertions[2].Equals = &equals }, "assertions[2].equals"},
	} {
		d := validData(t)
		test.change(&d)
		err := profile.Validate(d)
		if !errors.Is(err, profile.ErrBundle) || !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), test.want) {
			t.Errorf("%s: %v, want ErrBundle naming %s", name, err, test.want)
		}
	}
}
