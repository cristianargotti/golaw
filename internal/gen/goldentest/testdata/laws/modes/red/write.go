package write

import "os"

// Save writes one report.
func Save(dir string, data []byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(dir+"/report.txt", data, 0o644)
}
