package main

import (
	"path/filepath"
	"slices"
	"strings"
)

func m2Environment(parent []string, home, bin, cacheBase string) []string {
	env := isolatedEnvironment(parent, home, bin)
	env = slices.DeleteFunc(env, func(entry string) bool {
		key, _, _ := strings.Cut(entry, "=")
		return strings.EqualFold(key, "GOCACHE") || strings.EqualFold(key, "GOMODCACHE")
	})
	return append(env,
		"GOCACHE="+filepath.Join(cacheBase, "caches", "go-build"),
		"GOMODCACHE="+filepath.Join(cacheBase, "caches", "go-mod"))
}

func (s *suite) m2Scope(name string) *suite {
	base := filepath.Join(s.m2.Base, "cases", name)
	home := filepath.Join(base, "home")
	return &suite{
		root: s.root, base: base, home: home, binary: s.binary,
		git: s.git, goTool: s.goTool, digest: s.digest,
		execute: s.execute, lookup: s.lookup, m2: s.m2, m2OS: s.m2OS,
		m2Prerequisites: s.m2Prerequisites,
		env:             m2Environment(s.env, home, filepath.Dir(s.binary), s.m2.Base),
	}
}
