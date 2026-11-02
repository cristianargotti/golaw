package contract

import "errors"

// ErrNoPort is a symbol the contract file does not declare as an interface:
// the request names the wrong file or symbol, so the remedy is the flag, not
// the contract.
var ErrNoPort = errors.New("port not found")

// ErrNoServer is an oapi-codegen output without the strict server interface:
// the handler kind fills that interface alone, so the remedy is the strict
// server option of the generator, never a guess at the operations.
var ErrNoServer = errors.New("strict server interface not found")
