package run

import "os"

// Clean removes the temporary file of one run.
func Clean(path string) {
	_ = os.Remove(path) //nolint:errcheck // the red shape of the law
}
