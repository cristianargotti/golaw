package main

import "github.com/cristianargotti/golaw/internal/bootstrap"

// generatePolicy writes the generate block init detects into a new
// policy (M7.6.2); a reviewed policy keeps its bytes, as surfacePolicy
// does, and runs on the defaults until the person or init --update
// writes the block.
func generatePolicy(root string, data []byte, existing bool) ([]byte, error) {
	if existing {
		return data, nil
	}
	detected, err := bootstrap.DetectGenerate(root)
	if err != nil {
		return nil, initOperation("detect the generate layout", err)
	}
	written, err := bootstrap.SetGenerate(data, detected)
	if err != nil {
		return nil, initOperation("write the generate block", err)
	}
	return written, nil
}
