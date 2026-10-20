package bootstrap

import "strings"

func ownershipPatternProblem(pattern string) string {
	switch {
	case strings.HasPrefix(pattern, "!"):
		return "unsupported negation"
	case strings.ContainsAny(pattern, "[]"):
		return "unsupported bracket pattern"
	case strings.Contains(pattern, "?"):
		return "unsupported question wildcard"
	case strings.Contains(pattern, "\\"):
		return "unsupported escape syntax"
	}
	for part := range strings.SplitSeq(strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/"), "/") {
		if part == "" || part == "." || part == ".." {
			return "unsupported path component"
		}
		if strings.Contains(part, "**") && part != "**" {
			return "unsupported recursive wildcard"
		}
	}
	return ""
}

func ownershipMatches(pattern, destination string) bool {
	rooted, directory := strings.HasPrefix(pattern, "/"), strings.HasSuffix(pattern, "/")
	pattern = strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/")
	wanted, parts := strings.Split(pattern, "/"), strings.Split(destination, "/")
	if !directory && len(wanted) > 1 && wanted[len(wanted)-1] == "**" {
		return ownershipDescendant(wanted[:len(wanted)-1], parts)
	}
	// GitHub reads docs/* as the files of docs alone, so a final bare star
	// names no directory whose descendants it would also take.
	if !directory && (rooted || len(wanted) > 1) && wanted[len(wanted)-1] == "*" {
		return ownershipComponents(wanted, parts)
	}
	limit := len(parts)
	if directory {
		limit--
	}
	for end := 1; end <= limit; end++ {
		if len(wanted) == 1 && !rooted {
			if ownershipComponent(wanted[0], parts[end-1]) {
				return true
			}
			continue
		}
		if ownershipComponents(wanted, parts[:end]) {
			return true
		}
	}
	return false
}

func ownershipDescendant(pattern, destination []string) bool {
	for end := range destination {
		if ownershipComponents(pattern, destination[:end]) {
			return true
		}
	}
	return false
}

func ownershipComponents(pattern, destination []string) bool {
	matches := make([]bool, len(destination)+1)
	matches[0] = true
	for _, part := range pattern {
		next := make([]bool, len(matches))
		for index, matched := range matches {
			if part == "**" {
				next[index] = matched || index > 0 && next[index-1]
			} else if matched && index < len(destination) && ownershipComponent(part, destination[index]) {
				next[index+1] = true
			}
		}
		matches = next
	}
	return matches[len(destination)]
}

func ownershipComponent(pattern, name string) bool {
	first, rest, wildcard := strings.Cut(pattern, "*")
	if !wildcard {
		return pattern == name
	}
	if !strings.HasPrefix(name, first) {
		return false
	}
	name = strings.TrimPrefix(name, first)
	for {
		part, remaining, more := strings.Cut(rest, "*")
		if !more {
			return strings.HasSuffix(name, part)
		}
		index := strings.Index(name, part)
		if index < 0 {
			return false
		}
		name, rest = name[index+len(part):], remaining
	}
}
