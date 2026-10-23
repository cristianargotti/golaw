package main

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/profile"
)

// The pt-BR lines a person reads from the profile lifecycle (D20).

// installedLine names the version made current and where it came from, or
// that the store already held it with the same digest.
func installedLine(i profile.Installed, already bool) string {
	if already {
		return fmt.Sprintf("Perfil %s %s já estava instalado", i.Name, i.Version)
	}
	return fmt.Sprintf("Perfil %s %s instalado de %s@%s, digest %s", i.Name, i.Version, i.URL, i.Tag, i.Digest)
}

// rolledBackLine names the version made current again.
func rolledBackLine(i profile.Installed) string {
	return fmt.Sprintf("Perfil %s voltou para %s", i.Name, i.Version)
}

// removedLine names the profile removed with every stored version.
func removedLine(name string) string {
	return fmt.Sprintf("Perfil %s removido", name)
}
