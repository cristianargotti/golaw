package pg

import "fmt"

// byName is the generated query the repository binds a parameter to.
const byName = "SELECT id FROM users WHERE name = $1"

// ByName names the query and the parameter it receives.
func ByName(name string) (string, string) {
	return byName, fmt.Sprintf("user %s", name)
}
