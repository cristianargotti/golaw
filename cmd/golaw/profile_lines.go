package main

import (
	"fmt"

	"github.com/cristianargotti/golaw/internal/profile"
)

// The lines a person reads from the profile lifecycle (D20): the outcome
// in pt-BR, the language of the human channel, which no golaw.yml can name
// here because the lifecycle runs outside any repository; the detail line
// of the record in English.

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

// pinHintLine says how the pin follows a version made current, because no
// step of the lifecycle writes golaw.yml (A19): init does, committed.
func pinHintLine(i profile.Installed) string {
	return fmt.Sprintf("golaw.yml não mudou: para fixar %s %s em um repositório, execute golaw init --profile %s --apply e faça commit de golaw.yml",
		i.Name, i.Version, i.Name)
}

// installLines are the lines of an install: nothing more when the version
// was already current, otherwise the refresh lines and the pin line.
func installLines(i profile.Installed, already bool) []string {
	if already {
		return []string{installedLine(i, true)}
	}
	return append(refreshLines(i), pinHintLine(i))
}

// refreshLines are the lines of a version installed again, which needs no
// pin move: the outcome, and the canonical source the pin names when the
// URL was a mirror (D9).
func refreshLines(i profile.Installed) []string {
	lines := []string{installedLine(i, false)}
	if i.Source != i.URL {
		lines = append(lines, "  source: "+i.Source+" (the source golaw.yml pins)")
	}
	return lines
}

// rolledBackLines are the lines of a rollback.
func rolledBackLines(i profile.Installed) []string {
	return []string{rolledBackLine(i), pinHintLine(i)}
}
