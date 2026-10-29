package surface

import (
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
	detect "github.com/cristianargotti/golaw/internal/surface"
)

const (
	dockerFrom = "FROM"
	dockerUser = "USER"
	// finalUserTail is the fix every final-user message states.
	finalUserTail = " (Sonar S6471): end the final stage with USER and a numeric user other than 0, such as USER 65532:65532."
)

// DockerTagAndDigest denies a FROM reference that carries both a tag and a
// digest when the candidate introduces it (M6.7 check 8a): the digest
// alone pins the image, and the remote analyzer reports the pair.
func DockerTagAndDigest() model.Check {
	return check{
		spec:      spec("docker.tag-and-digest"),
		languages: []detect.Language{detect.Dockerfile},
		certain:   true,
		judge:     judgeTagAndDigest,
	}
}

// judgeTagAndDigest answers one finding per FROM whose reference holds
// both, at the instruction's line.
func judgeTagAndDigest(f source, _ model.Input) []model.Finding {
	var out []model.Finding
	for _, ins := range readInstructions(f.src) {
		if ins.keyword != dockerFrom || !tagAndDigest(fromReference(ins.args)) {
			continue
		}
		message := "writes FROM with a tag and a digest (Sonar S8431): keep only the digest and put the tag in a comment on the line above."
		out = append(out, f.finding("tag-and-digest", ins.line, message, sonarKey(detect.Dockerfile, "S8431")))
	}
	return out
}

// fromReference answers the image reference of a FROM: the first word
// that is no --name=value flag.
func fromReference(args string) string {
	for word := range strings.FieldsSeq(args) {
		if !strings.HasPrefix(word, "--") {
			return word
		}
	}
	return ""
}

// tagAndDigest reports a reference with a digest whose name holds a tag: a
// colon after the last slash, so a registry port is never a tag. A
// reference that holds an expansion is not judged, because golaw does not
// resolve build arguments.
func tagAndDigest(ref string) bool {
	if strings.Contains(ref, "$") {
		return false
	}
	name, _, digest := strings.Cut(ref, "@")
	if !digest {
		return false
	}
	return strings.Contains(name[strings.LastIndexByte(name, '/')+1:], ":")
}

// DockerFinalUser reports a final stage that runs as root (M6.7 check 8b);
// it is feedback, never a denial. Smallest behavior: a named user other
// than root passes, because the image may define it and no predicate reads
// the base image.
func DockerFinalUser() model.Check {
	return check{
		spec:      spec("docker.final-user"),
		languages: []detect.Language{detect.Dockerfile},
		judge:     judgeFinalUser,
	}
}

// judgeFinalUser reads the instructions after the last FROM: no USER is
// one finding at that FROM, a last USER of root or 0 one at that USER.
func judgeFinalUser(f source, _ model.Input) []model.Finding {
	var from, user *instruction
	instructions := readInstructions(f.src)
	for i := range instructions {
		switch instructions[i].keyword {
		case dockerFrom:
			from, user = &instructions[i], nil
		case dockerUser:
			user = &instructions[i]
		}
	}
	key := sonarKey(detect.Dockerfile, "S6471")
	switch {
	case from == nil:
		return nil
	case user == nil:
		return []model.Finding{f.finding("final-user", from.line, "the final stage sets no USER, so the container runs as root"+finalUserTail, key)}
	}
	name, _, _ := strings.Cut(user.args, ":")
	if name = strings.TrimSpace(name); name != "root" && name != "0" {
		return nil
	}
	return []model.Finding{f.finding("final-user", user.line, "the final stage runs as root through USER "+user.args+finalUserTail, key)}
}
