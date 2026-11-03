package genmap_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gen/manifest"
	"github.com/cristianargotti/golaw/internal/genmap"
	"github.com/cristianargotti/golaw/internal/model"
)

func shopContext(t *testing.T, in genmap.Input) genmap.Context {
	t.Helper()
	if in.Snapshot == nil {
		in.Snapshot = shopSnapshot(t, nil)
	}
	if in.Area == "" {
		in.Area = "shop"
	}
	if !in.Policy.Declared && in.Policy.Layout.Port == "" && in.Policy.Layout.Usecase == "" {
		in.Policy = model.DefaultGenerate()
	}
	c, err := genmap.BuildContext(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Each row is a red row, a shape the context must print, or a near row,
// a shape beside it that must stay out.
func TestContextPrintsEachLayerOfTheArea(t *testing.T) {
	t.Parallel()
	text := shopContext(t, genmap.Input{}).Text(1 << 20)
	rows := []struct {
		name, want string
		present    bool
	}{
		{"port with signatures", "shop.Orders internal/application/shop/ports.go:11\n  Get(ctx context.Context, id order.ID) (order.Order, error)\n  Save(ctx context.Context, o order.Order) error\n", true},
		{"near: an unexported interface", "hidden", false},
		{"use case", "shop.PlaceOrder internal/application/shop/place.go:21\n  new NewPlaceOrder(orders Orders) *PlaceOrder\n  Execute(ctx context.Context, in PlaceInput) (Placed, error)\n  in PlaceInput struct{Customer string; Lines []Line}\n  out Placed struct{ID string}\n  Describe() string\n", true},
		{"near: an unexported method", "unexported", false},
		{"constructor of the use cases", "shop.NewUseCases internal/application/shop/use_cases.go:17\n  NewUseCases(deps Deps) UseCases\n  in Deps struct{Store Orders; Time Clock; Prefs Settings}\n", true},
		{"sentinels", "shop errors internal/application/shop/errors.go:6\n  ErrNotFound\n", true},
		{"near: a variable that is no sentinel", "ErrNotFound, other", false},
		{"domain enum", "order.Status internal/domain/order/order.go:7\n  string\n  consts Open=\"open\", Closed=\"closed\"\n", true},
		{"iota enum", "order.Kind internal/domain/order/order.go:15\n  int\n  consts Small, Large\n", true},
		{"near: an untyped constant", "Untyped", false},
		{"type at the second depth", "order.Item internal/domain/order/order.go:32\n  struct{SKU string}\n", true},
		{"asserted adapter", "postgres.Orders internal/adapters/out/postgres/orders.go:18\n  new NewOrders() *Orders\n  port shop.Orders (asserted)\n  queries orders.sql\n", true},
		{"wiring adapter", "main.ledger cmd/app/main.go:12\n  port shop.Orders (methods)\n", true},
		{"near: one method of the port", "postgres.Audit", false},
		{"near: the method of a weak port", "postgres.Ticker", false},
		{"handler with routes", "shopapi.Handler internal/adapters/in/shopapi/handler.go:21\n  new New(deps Deps) *Handler\n  in Deps struct{Uses shop.UseCases}\n", true},
		{"call route", "  POST /shop/orders/{id} -> cancel handler.go:50\n", true},
		{"free function route", "  * /shop/ -> notFound\n", true},
		{"router verb", "  GET /shop/health -> health handler.go:52\n", true},
		{"literal route with its use case", "  GET /shop/orders -> place handler.go:46 uses PlaceOrder\n", true},
		{"method that calls a use case", "shopapi.Strict internal/adapters/in/shopapi/handler.go:57\n  PlaceOrder(ctx context.Context) error uses PlaceOrder\n", true},
		{"near: a pattern that is no route", "not a route", false},
		{"near: a package that never imports the area", "other.H", false},
		{"table after its alter", "orders db/migrations/0001_init.sql:2\n  id TEXT pk, customer TEXT, total NUMERIC(10, 2) null, placed_at TIMESTAMPTZ\n", true},
		{"quoted table", "audit db/migrations/0001_init.sql:9\n  id TEXT null\n", true},
		{"near: a dropped column", "status VARCHAR", false},
		{"near: a table of the down part", "gone", false},
		{"near: a table never closed", "broken", false},
		{"queries of the area", "db/queries/orders.sql\n  GetOrder :one, SaveOrder :exec, Joined :many\n", true},
		{"near: queries of another area", "billing.sql", false},
		{"wiring call", "shop.NewUseCases cmd/app/main.go:20\n", true},
		{"wiring literal", "ledger{} cmd/app/main.go:20\n", true},
		{"near: an empty literal", "shop.Deps{} cmd/app/main.go:22", false},
		{"pattern that mentions the layer", "usecase shop internal/application/shop/place_test.go\n", true},
		{"adapter pattern", "adapter postgres internal/adapters/out/postgres/orders_test.go\n", true},
		{"near: a TestMain alone", "postgres/main_test.go", false},
		{"near: a test behind a build tag", "integration_test.go", false},
		{"port command", "golaw new port shop.<Port>: adopts", true},
		{"near: the use case command of a hand written area", "golaw new usecase", false},
	}
	for _, row := range rows {
		if got := strings.Contains(text, row.want); got != row.present {
			t.Errorf("%s: contains %q = %v, want %v in\n%s", row.name, row.want, got, row.present, text)
		}
	}
}

func TestContextOfAGeneratedAreaAddsItsQueriesAndCommands(t *testing.T) {
	t.Parallel()
	m := &manifest.Manifest{Schema: manifest.Schema, Entries: []manifest.Entry{{
		Kind: genkind.KindUsecase, Template: "usecase@1", Name: "shop.PlaceOrder",
		Contracts: []manifest.Contract{{Path: "db/queries/billing.sql"}},
	}}}
	text := shopContext(t, genmap.Input{Manifest: m}).Text(1 << 20)
	for _, want := range []string{"db/queries/billing.sql\n  Bill :one\n", "golaw new usecase shop.<Name>", "golaw new handler <pkg> --usecases shop"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

func TestContextRefusesAnUnknownAreaAndACanceledContext(t *testing.T) {
	t.Parallel()
	in := genmap.Input{Snapshot: shopSnapshot(t, nil), Policy: model.DefaultGenerate(), Area: "billing"}
	if _, err := genmap.BuildContext(t.Context(), in); !errors.Is(err, genmap.ErrNoArea) {
		t.Fatalf("unknown area: %v", err)
	}
	if got := genmap.Areas(in); !slices.Equal(got, []string{"shop"}) {
		t.Errorf("areas = %v", got)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	in.Area = "shop"
	if _, err := genmap.BuildContext(ctx, in); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
}

// Near row: without go.mod no import resolves, so the domain types of
// another package stay out while the area's own shapes remain.
func TestContextWithoutAModuleKeepsTheAreaOnly(t *testing.T) {
	t.Parallel()
	files := map[string]string{"go.mod": "go 1.27\n"}
	p := model.DefaultGenerate()
	p.Layout.Port = ""
	text := shopContext(t, genmap.Input{Snapshot: shopSnapshot(t, files), Policy: p}).Text(1 << 20)
	if !strings.Contains(text, "shop.Line internal/application/shop/place.go:12") || strings.Contains(text, "order.Status") {
		t.Errorf("text:\n%s", text)
	}
}

func TestContextTextDropsTheLastSectionsFirst(t *testing.T) {
	t.Parallel()
	c := shopContext(t, genmap.Input{})
	whole := c.Text(1 << 20)
	if strings.Contains(whole, " more ") {
		t.Fatalf("a budget that fits everything still omits:\n%s", whole)
	}
	cut := c.Text((len(whole) - 1) / 4)
	if !strings.Contains(cut, "... 1 more commands: golaw context shop --json\n") || strings.Contains(cut, "more patterns") {
		t.Errorf("one byte over drops more than the commands:\n%s", cut)
	}
	if len(cut) > len(whole) {
		t.Errorf("cut has %d bytes, more than the whole %d", len(cut), len(whole))
	}
	tight := c.Text(200)
	if len(tight) > 800 || !strings.Contains(tight, " more types: golaw context shop --json\n") || !strings.Contains(tight, "shop.Orders") {
		t.Errorf("tight (%d bytes):\n%s", len(tight), tight)
	}
}

func TestContextJSONWritesEverySectionAsAnArray(t *testing.T) {
	t.Parallel()
	for _, c := range []genmap.Context{{}, shopContext(t, genmap.Input{})} {
		data, err := c.JSON()
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"ports", "usecases", "types", "adapters", "handlers", "tables", "queries", "wiring", "patterns", "commands"} {
			if raw := doc[key]; len(raw) == 0 || raw[0] != '[' {
				t.Errorf("%s = %s, want an array", key, raw)
			}
		}
	}
}
