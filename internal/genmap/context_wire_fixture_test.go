package genmap_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/gentest"
	"github.com/cristianargotti/golaw/internal/model"
)

// The outer layers of the shop fixture: the handler package, the wiring
// root, the migration and the queries.
const (
	shopHandler = `package shopapi

import (
	"context"
	"net/http"

	"example.com/slice/internal/application/shop"
)

const (
	prefix     = "/shop"
	pathOrders = prefix + "/orders"
	pathHealth = (prefix + "/health")
	get        = "GET "
)

// Deps are what the routes need.
type Deps struct{ Uses shop.UseCases }

// Handler serves the shop.
type Handler struct{ deps Deps }

type route struct {
	pattern string
	handler http.HandlerFunc
}

type router interface{ Get(pattern string, h http.HandlerFunc) }

// New builds the handler.
func New(deps Deps) *Handler { return &Handler{deps: deps} }

// Mount registers the routes.
func (h *Handler) Mount(mux *http.ServeMux, r router) {
	for _, rt := range h.routes() {
		mux.Handle(rt.pattern, rt.handler)
	}
	mux.HandleFunc("POST /shop/orders/{id}", h.cancel)
	mux.Handle("/shop/", http.HandlerFunc(notFound))
	r.Get(pathHealth, h.health)
	mux.Handle("not a route", h.cancel)
}

func (h *Handler) routes() []route { return []route{{get + pathOrders, h.place}} }

func (h *Handler) place(w http.ResponseWriter, r *http.Request) {
	_, _ = h.deps.Uses.Place.Execute(r.Context(), shop.PlaceInput{})
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {}

func notFound(w http.ResponseWriter, r *http.Request) {}

// Strict serves the shop through a generated interface.
type Strict struct{ uses shop.UseCases }

// PlaceOrder places an order.
func (s Strict) PlaceOrder(ctx context.Context) error {
	_, err := s.uses.Place.Execute(ctx, shop.PlaceInput{})
	return err
}
`
	shopMain = `package main

import (
	"context"

	"example.com/slice/internal/adapters/in/shopapi"
	"example.com/slice/internal/adapters/out/postgres"
	"example.com/slice/internal/application/shop"
	"example.com/slice/internal/domain/order"
)

type ledger struct{ inner *postgres.Orders }

func (l ledger) Get(ctx context.Context, id order.ID) (order.Order, error) { return l.inner.Get(ctx, id) }

func (l ledger) Save(ctx context.Context, o order.Order) error { return nil }

func main() {
	store := postgres.NewOrders()
	uses := shop.NewUseCases(shop.Deps{Store: ledger{inner: store}, Prefs: shop.Settings{Currency: "BRL"}})
	_ = shopapi.New(shopapi.Deps{Uses: uses})
	_ = shop.Deps{}
}
`
	shopMigration = `-- +goose Up
CREATE TABLE orders (
    id TEXT PRIMARY KEY,
    customer TEXT NOT NULL, -- who, (with a comma)
    total NUMERIC(10, 2),
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    CONSTRAINT orders_total CHECK (total >= 0)
);
CREATE TABLE IF NOT EXISTS "public"."audit" (id TEXT);
ALTER TABLE orders ADD COLUMN placed_at TIMESTAMPTZ NOT NULL, DROP COLUMN status;
ALTER TABLE missing ADD COLUMN x INT;
CREATE TABLE broken (id TEXT
-- +goose Down
DROP TABLE orders;
CREATE TABLE gone (id INT);
`
	shopQueries = `-- name: GetOrder :one
SELECT id FROM orders WHERE id = $1;

-- name: SaveOrder :exec
INSERT INTO orders (id) VALUES ($1);

-- name: Joined :many
SELECT o.id FROM orders o JOIN "audit" a ON a.id = o.id LIMIT 10; -- FROM nowhere
`
)

// shopFiles are the sources of the fixture by slash path.
func shopFiles() map[string]string {
	return map[string]string{
		"go.mod":                                                    "module example.com/slice\n\ngo 1.27\n",
		"internal/application/shop/ports.go":                        shopPorts,
		"internal/application/shop/errors.go":                       shopErrors,
		"internal/application/shop/place.go":                        shopPlace,
		"internal/application/shop/use_cases.go":                    shopUses,
		"internal/application/shop/place_test.go":                   "package shop_test\n\nimport \"testing\"\n\nfunc TestPlace(t *testing.T) { _ = \"Execute(\" }\n",
		"internal/application/shop/tiny_test.go":                    "package shop_test\n\nimport \"testing\"\n\nfunc TestTiny(t *testing.T) {}\n",
		"internal/application/shop/broken.go":                       "package shop\n\nfunc (\n",
		"internal/domain/order/order.go":                            orderDomain,
		"internal/adapters/out/postgres/orders.go":                  postgresOrders,
		"internal/adapters/out/postgres/orders_test.go":             "package postgres_test\n\nimport \"testing\"\n\n// Orders is tested here.\nfunc TestOrders(t *testing.T) {}\n",
		"internal/adapters/out/postgres/main_test.go":               "package postgres_test\n\nimport \"testing\"\n\nfunc TestMain(m *testing.M) {}\n",
		"internal/adapters/out/postgres/orders_integration_test.go": "//go:build integration\n\npackage postgres_test\n\nimport \"testing\"\n\nfunc TestIt(t *testing.T) {}\n",
		"internal/adapters/in/shopapi/handler.go":                   shopHandler,
		"internal/adapters/in/other/other.go":                       "package other\n\nfunc (h *H) place() {}\n\ntype H struct{}\n",
		"cmd/app/main.go":                                           shopMain,
		"cmd/app/main_test.go":                                      "package main\n\nimport \"testing\"\n\nfunc TestMainRuns(t *testing.T) {}\n",
		"db/migrations/0001_init.sql":                               shopMigration,
		"db/queries/orders.sql":                                     shopQueries,
		"db/queries/billing.sql":                                    "-- name: Bill :one\nSELECT 1 FROM bills;\n",
	}
}

// shopSnapshot is the fixture as a snapshot, with extra files over it.
func shopSnapshot(t *testing.T, extra map[string]string) model.Snapshot {
	t.Helper()
	files := map[string][]byte{}
	for p, text := range shopFiles() {
		files[p] = []byte(text)
	}
	for p, text := range extra {
		files[p] = []byte(text)
	}
	return gentest.Snapshot(t, files)
}
