package gen_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/holes"
	"github.com/cristianargotti/golaw/internal/source"
)

func regenerating(appends ...genkind.Append) fakeKind {
	out := plansOutput()
	out.Appends = appends
	return fakeKind{name: genkind.KindAdapterSQL, regen: out}
}

func TestGenerateAppendsTheMissingStubsAndKeepsWhatMatches(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	res, err := gen.Generate(t.Context(), engineInput(t, inSync(t, m), m, supportKind(), regenerating(archiveAppend())))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Writes) != 1 || res.Writes[0].Path != plansPath || res.Writes[0].Mode != gen.WriteAppend ||
		res.Writes[0].Base != source.ContentHash([]byte(ownedPlans)) {
		t.Fatalf("writes = %+v, want the append alone (the version token drifts nothing)", res.Writes)
	}
	found, err := holes.Find(plansPath, res.Writes[0].Content)
	if err != nil || len(found) != 2 || strings.Count(string(res.Writes[0].Content), "func ReadPlan") != 1 {
		t.Fatalf("appended holes = %+v, %v", found, err)
	}
	if len(res.Opened) != 1 || res.Opened[0].ID != "plans.archive-plan" || res.Closed != nil || len(res.Open) != 2 {
		t.Errorf("opened %+v, closed %+v, open %+v", res.Opened, res.Closed, res.Open)
	}
}

func TestGenerateRewritesAHandEditedRegenerableFile(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	files := inSync(t, m)
	files[plansGen] = append(plansGenerated("0.3.0"), []byte("// a hand edit\n")...)
	res, err := gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), regenerating()))
	want := []gen.Write{{Path: plansGen, Content: plansGenerated("0.3.0"), Mode: gen.WriteRegenerable, Base: source.ContentHash(files[plansGen])}}
	if err != nil || !reflect.DeepEqual(res.Writes, want) {
		t.Errorf("Generate = %+v, %v", res.Writes, err)
	}
}

func TestGenerateRegeneratesOnlyTheEntriesOfTheChangedContracts(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	files := inSync(t, m)
	delete(files, plansGen)
	delete(files, holePath)
	in := engineInput(t, files, m, supportKind(), regenerating())
	in.Changed = []string{"db/queries/plans.sql"}
	if res, err := gen.Generate(t.Context(), in); err != nil || len(res.Writes) != 0 {
		t.Errorf("a change no entry contracts = %+v, %v", res.Writes, err)
	}
	in.Changed = []string{portsPath}
	res, err := gen.Generate(t.Context(), in)
	if err != nil || len(res.Writes) != 1 || res.Writes[0].Path != plansGen {
		t.Errorf("a contract change = %+v, %v, want the entry's file and no support or manifest write", res.Writes, err)
	}
}

func TestGenerateNamesAGoneContractAndRefusesAnUnknownTemplate(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	files := inSync(t, m)
	delete(files, portsPath)
	res, err := gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), regenerating()))
	if err != nil || len(res.Decisions) != 1 || res.Decisions[0].Path != portsPath || !strings.Contains(res.Decisions[0].Text, "is gone") {
		t.Errorf("a gone contract = %+v, %v", res.Decisions, err)
	}
	newer := plansEntry()
	newer.Template = "adapter-sql@2"
	m2 := enrolled(newer)
	if _, err := gen.Generate(t.Context(), engineInput(t, inSync(t, m2), m2, supportKind(), regenerating())); !errors.Is(err, gen.ErrUnknownTemplate) ||
		!strings.Contains(err.Error(), "golaw gen --upgrade") || !strings.Contains(err.Error(), "postgres.Plans") {
		t.Errorf("an unknown template = %v", err)
	}
	if _, err := gen.Generate(t.Context(), engineInput(t, inSync(t, m), m, supportKind())); !errors.Is(err, gen.ErrUnknownKind) {
		t.Errorf("a kind the kit lacks = %v", err)
	}
}

func TestGenerateNeverWritesIntoAHandwrittenFile(t *testing.T) {
	t.Parallel()
	m := enrolled(plansEntry())
	files := inSync(t, m)
	files[plansPath] = []byte("// golaw:handwritten the vendor API streams a chunked body\n" + ownedPlans)
	files[plansGen] = []byte("// golaw:handwritten the queries are tuned by hand\npackage postgres\n")
	res, err := gen.Generate(t.Context(), engineInput(t, files, m, supportKind(), regenerating(archiveAppend())))
	if err != nil || len(res.Writes) != 0 {
		t.Errorf("Generate wrote into a handwritten file: %+v, %v", res.Writes, err)
	}
}
