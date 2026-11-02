package contract_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

const portsPath = "internal/application/growth/ports.go"

func growthReader(t *testing.T) contract.Reader {
	t.Helper()
	return contract.Reader{Snapshot: gentest.Snapshot(t, gentest.Contracts())}
}

func TestReaderPortReadsTheGrowthLedger(t *testing.T) {
	t.Parallel()
	port, err := growthReader(t).Port(portsPath, "Ledger")
	if err != nil {
		t.Fatal(err)
	}
	ctx := genkind.Param{Name: "ctx", Type: "context.Context"}
	account := genkind.Param{Name: "account", Type: "AccountID"}
	want := genkind.Port{
		Name: "Ledger", Package: "growth", Path: portsPath, Dir: "internal/application/growth",
		Doc: "Ledger stores the plans of an account.", Line: 10,
		Imports: []genkind.Import{{Path: "context"}, {Path: "time"}},
		Methods: []genkind.Method{
			{
				Name: "ReadPlan", Doc: "ReadPlan reads the plan of one account.", Line: 12,
				Params: []genkind.Param{ctx, account}, Results: []genkind.Param{{Type: "Plan"}, {Type: "error"}},
			},
			{
				Name: "ArchivePlan", Doc: "ArchivePlan archives the plan of one account.", Line: 14,
				Params: []genkind.Param{ctx, account}, Results: []genkind.Param{{Type: "error"}},
			},
		},
	}
	if !reflect.DeepEqual(port, want) {
		t.Errorf("Port = %+v\nwant   %+v", port, want)
	}
}

func TestPortsListsEveryInterfaceInSourceOrder(t *testing.T) {
	t.Parallel()
	ports, err := contract.Ports(portsPath, gentest.Contracts()[portsPath])
	if err != nil {
		t.Fatal(err)
	}
	if len(ports) != 2 || ports[0].Name != "Ledger" || ports[1].Name != "Clock" {
		t.Fatalf("Ports = %+v", ports)
	}
	now := []genkind.Method{{Name: "Now", Doc: "Now answers the current time.", Line: 20, Results: []genkind.Param{{Type: "time.Time"}}}}
	if !reflect.DeepEqual(ports[1].Methods, now) || ports[1].Line != 18 {
		t.Errorf("Clock = %+v", ports[1])
	}
}

// The red rows: every shape a reader without types cannot judge, each
// refused with its construct, its symbol and its line.
func TestPortsRefuseTheShapesThatNeedTypes(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		src, symbol, want string
	}{
		"generic interface":  {"package p\n\ntype Store[T any] interface {\n\tGet() T\n}\n", "Store", "p.go:3 generic interface in Store"},
		"alias":              {"package p\n\nimport \"io\"\n\ntype Store = io.Reader\n", "Store", "p.go:5 alias in Store"},
		"embedded interface": {"package p\n\nimport \"io\"\n\ntype Store interface {\n\tio.Closer\n\tGet() error\n}\n", "Store", "p.go:6 embedded io.Closer in Store"},
		"type set":           {"package p\n\ntype Number interface {\n\t~int | ~float64\n}\n", "Number", "p.go:4 type set in Number"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ports, err := contract.Ports("p.go", []byte(tc.src))
			if len(ports) != 0 || !errors.Is(err, genkind.ErrCannotJudge) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Ports = %+v, %v; want a refusal naming %q", ports, err, tc.want)
			}
			r := contract.Reader{Snapshot: gentest.Snapshot(t, map[string][]byte{"p.go": []byte(tc.src)})}
			if _, err := r.Port("p.go", tc.symbol); !errors.Is(err, genkind.ErrCannotJudge) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Port = %v, want the same refusal", err)
			}
		})
	}
}

// The near misses: unnamed and variadic parameters, a generic type
// argument, two names of one type and a method whose name says embedded
// are all judged; a struct beside the interface is no port.
func TestPortsJudgeTheNearMisses(t *testing.T) {
	t.Parallel()
	src := "package p\n\nimport \"context\"\n\n// Store keeps lists.\ntype Store interface {\n" +
		"\tPut(context.Context, ...string) error\n" +
		"\tList(ctx context.Context, page Page[int]) ([]string, int, error)\n" +
		"\tCopy(dst, src string) error\n" +
		"\tEmbeddedName() Embedded\n}\n\ntype Plain struct{}\n"
	ports, err := contract.Ports("p.go", []byte(src))
	if err != nil || len(ports) != 1 || ports[0].Doc != "Store keeps lists." {
		t.Fatalf("Ports = %+v, %v", ports, err)
	}
	str := genkind.Param{Type: "string"}
	errType := genkind.Param{Type: "error"}
	want := []genkind.Method{
		{Name: "Put", Line: 7, Params: []genkind.Param{{Type: "context.Context"}, {Type: "...string"}}, Results: []genkind.Param{errType}},
		{
			Name: "List", Line: 8,
			Params:  []genkind.Param{{Name: "ctx", Type: "context.Context"}, {Name: "page", Type: "Page[int]"}},
			Results: []genkind.Param{{Type: "[]string"}, {Type: "int"}, errType},
		},
		{Name: "Copy", Line: 9, Params: []genkind.Param{{Name: "dst", Type: str.Type}, {Name: "src", Type: str.Type}}, Results: []genkind.Param{errType}},
		{Name: "EmbeddedName", Line: 10, Results: []genkind.Param{{Type: "Embedded"}}},
	}
	if !reflect.DeepEqual(ports[0].Methods, want) {
		t.Errorf("Methods = %+v\nwant      %+v", ports[0].Methods, want)
	}
}

func TestReaderPortNamesWhatIsMissing(t *testing.T) {
	t.Parallel()
	r := contract.Reader{Snapshot: gentest.Snapshot(t, map[string][]byte{
		"p.go":   []byte("package p\n\ntype Plain struct{}\n"),
		"bad.go": []byte("package p\n\nfunc (\n"),
	})}
	if _, err := r.Port("p.go", "Missing"); !errors.Is(err, contract.ErrNoPort) || !strings.Contains(err.Error(), "Missing") {
		t.Errorf("an absent symbol = %v, want ErrNoPort naming it", err)
	}
	if _, err := r.Port("p.go", "Plain"); !errors.Is(err, contract.ErrNoPort) || !strings.Contains(err.Error(), "not an interface") {
		t.Errorf("a struct = %v, want ErrNoPort saying it is not an interface", err)
	}
	if _, err := r.Port("bad.go", "X"); err == nil || errors.Is(err, genkind.ErrCannotJudge) || !strings.Contains(err.Error(), "bad.go") {
		t.Errorf("a file that does not parse = %v, want a parse error naming it", err)
	}
	if _, err := r.Port("absent.go", "X"); !errors.Is(err, model.ErrNotInSnapshot) {
		t.Errorf("an absent file = %v, want ErrNotInSnapshot", err)
	}
}
