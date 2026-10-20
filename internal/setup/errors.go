// Package setup keeps evidence of the local hooks a developer chose to arm.
package setup

import (
	"errors"
	"fmt"
)

// ErrReceipt prevents corrupt ownership evidence from authorizing changes.
var ErrReceipt = errors.New("invalid setup receipt")

func receiptError(path string, err error) error {
	return fmt.Errorf("setup receipt %q: %w", path, err)
}
