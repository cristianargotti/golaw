package profile

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// tagPattern is the tag a bundle is installed from, vX.Y.Z or X.Y.Z, the
// version being the exact X.Y.Z the stamp must carry.
var tagPattern = regexp.MustCompile(`^v?((0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*))$`)

// Source is where a person installs a bundle from: a git URL the user's
// git can clone, the tag, and the version the tag names.
type Source struct {
	URL, Tag, Version string
}

// Installed is one installed version of a bundle, as the install record
// keeps it: the identity of its stamp, where it came from and when.
type Installed struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Digest      string    `json:"digest"`
	Source      string    `json:"source"` // the stamp's canonical source, what golaw.yml pins
	URL         string    `json:"url"`    // the URL the person gave, a mirror allowed
	Tag         string    `json:"tag"`
	InstalledAt time.Time `json:"installed_at"`
}

// ParseSource splits "<git url>@<tag>" at its last @, so an ssh URL with a
// user keeps its own @; the tag must be vX.Y.Z or X.Y.Z. Anything else is
// a usage error naming the form, and an ssh URL given without its tag is
// named whole, because what follows its @ is a host, never a tag.
func ParseSource(arg string) (Source, error) {
	at := strings.LastIndex(arg, "@")
	if at <= 0 || at == len(arg)-1 || strings.ContainsAny(arg[at+1:], ":/") {
		return Source{}, fmt.Errorf("%w: %q is not <git url>@<tag>", model.ErrUsage, arg)
	}
	url, tag := arg[:at], arg[at+1:]
	match := tagPattern.FindStringSubmatch(tag)
	if match == nil {
		return Source{}, fmt.Errorf("%w: tag %q is not vX.Y.Z or X.Y.Z", model.ErrUsage, tag)
	}
	return Source{URL: url, Tag: tag, Version: match[1]}, nil
}
