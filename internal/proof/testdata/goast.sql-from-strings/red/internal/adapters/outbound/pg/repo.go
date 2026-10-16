package pg

import "fmt"

// ByName builds the query from the name itself.
func ByName(name string) string {
	return fmt.Sprintf("SELECT id FROM users WHERE name = '%s'", name)
}
