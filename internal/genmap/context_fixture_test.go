package genmap_test

// The shop fixture: one hand written area with its ports, use cases and
// domain package, an SQL adapter, a handler package, a wiring root, its
// migrations and queries, and the near rows beside each: a port no type
// fits, a weak clock, a package that never imports the area, a query file
// of another area and a test file the patterns must skip.
const (
	shopPorts = `package shop

import (
	"context"
	"time"

	"example.com/slice/internal/domain/order"
)

// Orders stores the orders.
type Orders interface {
	Get(ctx context.Context, id order.ID) (order.Order, error)
	Save(ctx context.Context, o order.Order) error
}

// Clock tells the time.
type Clock interface {
	Now() time.Time
}

type hidden interface{ x() }
`
	shopErrors = `package shop

import "errors"

// ErrNotFound is an unknown order.
var ErrNotFound = errors.New("not found")

var other = 1
`
	shopPlace = `package shop

import "context"

// PlaceInput is the order to place.
type PlaceInput struct {
	Customer string
	Lines    []Line
}

// Line is one line of an order.
type Line struct {
	SKU string
	Qty int
}

// Placed is the placed order.
type Placed struct{ ID string }

// PlaceOrder places an order.
type PlaceOrder struct{ orders Orders }

// NewPlaceOrder builds the use case.
func NewPlaceOrder(orders Orders) *PlaceOrder { return &PlaceOrder{orders: orders} }

// Describe names the use case.
func (u *PlaceOrder) Describe() string { return "place" }

// Execute places the order.
func (u *PlaceOrder) Execute(ctx context.Context, in PlaceInput) (Placed, error) { return Placed{}, nil }

func (u *PlaceOrder) unexported() {}
`
	shopUses = `package shop

// Deps are the ports of the use cases.
type Deps struct {
	Store Orders
	Time  Clock
	Prefs Settings
}

// Settings are the choices of the shop.
type Settings struct{ Currency string }

// UseCases are the use cases of the shop.
type UseCases struct{ Place *PlaceOrder }

// NewUseCases wires the use cases.
func NewUseCases(deps Deps) UseCases { return UseCases{Place: NewPlaceOrder(deps.Store)} }
`
	orderDomain = `package order

// ID names an order.
type ID string

// Status is the state of an order.
type Status string

const (
	Open   Status = "open"
	Closed        = Status("closed")
)

// Kind is the size of an order.
type Kind int

const (
	Small Kind = iota
	Large
	Untyped = 3
)

// Order is one order.
type Order struct {
	Key   ID
	State Status
	Size  Kind
	Items []Item
}

// Item is one item of an order.
type Item struct{ SKU string }
`
	postgresOrders = `package postgres

import (
	"context"
	"time"

	"example.com/slice/internal/application/shop"
	"example.com/slice/internal/domain/order"
)

// Queries stands for the sqlc queries.
type Queries struct{}

// GetOrder reads one order.
func (q *Queries) GetOrder(ctx context.Context, id string) (string, error) { return "", nil }

// Orders is the order store.
type Orders struct{ queries *Queries }

// NewOrders builds the store.
func NewOrders() *Orders { return &Orders{} }

// Get reads an order.
func (o *Orders) Get(ctx context.Context, id order.ID) (order.Order, error) {
	_, err := o.queries.GetOrder(ctx, string(id))
	return order.Order{}, err
}

// Save writes an order.
func (o *Orders) Save(ctx context.Context, x order.Order) error { return nil }

var _ shop.Orders = (*Orders)(nil)

// Audit has one method of the port only.
type Audit struct{}

// Get reads an audit line.
func (Audit) Get(ctx context.Context, id order.ID) (order.Order, error) { return order.Order{}, nil }

// Ticker has the clock's method, which is too plain to judge by.
type Ticker struct{}

// Now answers the time.
func (Ticker) Now() time.Time { return time.Time{} }
`
)
