package policyfiles

import (
	"encoding/hex"
	"path"
	"strings"
)

type dockerfile struct {
	path   string
	stages []dockerStage
	refs   []dockerRef
}

type dockerStage struct {
	from   string
	digest string
	name   string
	env    []dockerVar
	line   int
}

type dockerVar struct {
	name, value string
	line        int
}

// readDockerfile reads the subset the image and toolchain checks judge:
// FROM with an optional --platform, never expanded because it names the
// platform and not the image, and an optional AS; global ARG defaults
// substituted as $NAME or ${NAME} in FROM; the ENV of each stage in order;
// the images the leading --from of COPY and the from of a RUN --mount name.
// Lines are joined, split and freed of heredoc bodies as BuildKit's parser
// does, so a Windows checkout reads the instructions the pipeline reads.
func readDockerfile(file string, data []byte) (dockerfile, error) {
	doc := dockerfile{path: file}
	args := make(map[string]string)
	lines := strings.Split(strings.TrimPrefix(string(data), "\ufeff"), "\n")
	for index := range lines {
		lines[index] = strings.TrimSuffix(lines[index], "\r")
	}
	start, err := dockerPreamble(file, lines)
	if err != nil {
		return dockerfile{}, err
	}
	for index := start; index < len(lines); index++ {
		if err := doc.parseLine(lines, &index, args); err != nil {
			return dockerfile{}, err
		}
	}
	return doc, nil
}

func (d *dockerfile) parseLine(lines []string, index *int, args map[string]string) error {
	text, line := strings.TrimSpace(lines[*index]), *index+1
	if text == "" || strings.HasPrefix(text, "#") {
		return nil
	}
	text, complete := dockerLogicalLine(lines, index)
	if !complete {
		return outside(d.path, line, "unfinished backslash continuation")
	}
	if err := d.instruction(text, line, args); err != nil {
		return err
	}
	if !dockerBodies(lines, index, text) {
		return outside(d.path, line, "unsupported or unterminated heredoc")
	}
	return nil
}

func dockerPreamble(file string, lines []string) (int, error) {
	directives := true
	for index, raw := range lines {
		text := strings.TrimSpace(raw)
		if text != "" && !strings.HasPrefix(text, "#") {
			return index, nil
		}
		if directives {
			var err error
			directives, err = dockerDirective(file, index+1, text)
			if err != nil {
				return 0, err
			}
		}
	}
	return len(lines), nil
}

func dockerCommand(text string) (string, string) {
	text = strings.TrimSpace(text)
	end := strings.IndexAny(text, dockerSpace)
	if end < 0 {
		return strings.ToUpper(text), ""
	}
	return strings.ToUpper(text[:end]), strings.TrimSpace(text[end:])
}

func dockerDirective(file string, line int, text string) (bool, error) {
	key, value, found := strings.Cut(strings.TrimSpace(strings.TrimPrefix(text, "#")), "=")
	if !found {
		return false, nil
	}
	key, value = strings.ToLower(strings.TrimSpace(key)), strings.TrimSpace(value)
	switch key {
	case "escape":
		if value != "\\" {
			return false, outside(file, line, "unsupported escape parser directive "+value)
		}
	case "syntax":
		if !dockerOfficialFrontend(value) {
			return false, outside(file, line, "unsupported syntax parser directive "+value)
		}
	case "check":
	default:
		return false, nil
	}
	return true, nil
}

func dockerOfficialFrontend(reference string) bool {
	name, _, _ := strings.Cut(reference, "@")
	name, _, _ = strings.Cut(name, ":")
	return name == "docker/dockerfile" || name == "docker.io/docker/dockerfile"
}

func (d *dockerfile) instruction(text string, line int, args map[string]string) error {
	command, rest := dockerCommand(text)
	if (command == dockerCopy || command == dockerRun) && len(d.stages) != 0 {
		d.reference(command, rest, line)
		return nil
	}
	if command != "FROM" && command != "ENV" && (command != "ARG" || len(d.stages) != 0) {
		return nil
	}
	if command == "ARG" {
		dockerArguments(rest, args)
		return nil
	}
	words, ok := dockerWords(rest)
	if !ok {
		return outside(d.path, line, "unsupported quoting in "+command)
	}
	switch command {
	case "FROM":
		stage, err := dockerFrom(d.path, line, rest, words, args)
		if err != nil {
			return err
		}
		d.stages = append(d.stages, stage)
	case "ENV":
		return d.environment(words, line)
	}
	return nil
}

func dockerFrom(file string, line int, raw string, words []string, args map[string]string) (dockerStage, error) {
	const aliasedWords = 3
	if len(words) > 0 && strings.HasPrefix(words[0], "--platform=") {
		words = words[1:]
		_, raw, _ = dockerToken(raw)
		raw = strings.TrimSpace(raw)
	}
	if len(words) != 1 && (len(words) != aliasedWords || !strings.EqualFold(words[1], "AS")) {
		return dockerStage{}, outside(file, line, "unsupported FROM instruction")
	}
	if strings.HasPrefix(words[0], "--") {
		return dockerStage{}, outside(file, line, "unsupported FROM flag "+words[0])
	}
	_, tail, _ := dockerToken(raw)
	imageWord := raw[:len(raw)-len(tail)]
	if strings.Contains(imageWord, "$") && strings.ContainsAny(imageWord, "'\\") {
		return dockerStage{}, outside(file, line, "quoted or escaped parameter in FROM")
	}
	image, ok := dockerExpand(words[0], args)
	if !ok || image == "" || strings.ContainsAny(image, " \t\r\n") {
		return dockerStage{}, outside(file, line, "unresolved argument or parameter expression in FROM "+words[0])
	}
	stage := dockerStage{from: image, digest: dockerDigest(image), line: line}
	if len(words) == aliasedWords {
		stage.name = words[2]
	}
	return stage, nil
}

func (d *dockerfile) environment(words []string, line int) error {
	if len(words) == 0 || len(words) == 1 && !strings.Contains(words[0], "=") {
		return outside(d.path, line, "ENV without a value")
	}
	var values []dockerVar
	if !strings.Contains(words[0], "=") {
		values = []dockerVar{{name: words[0], value: strings.Join(words[1:], " "), line: line}}
	} else {
		for _, word := range words {
			name, value, assigned := strings.Cut(word, "=")
			if !assigned || name == "" {
				return outside(d.path, line, "unsupported ENV assignment "+word)
			}
			values = append(values, dockerVar{name: name, value: value, line: line})
		}
	}
	if len(d.stages) != 0 {
		stage := &d.stages[len(d.stages)-1]
		stage.env = append(stage.env, values...)
	}
	return nil
}

func dockerDigest(image string) string {
	const digestLength = 64
	_, digest, _ := strings.Cut(image, "@")
	encoded, ok := strings.CutPrefix(digest, "sha256:")
	if !ok || len(encoded) != digestLength {
		return ""
	}
	if _, err := hex.DecodeString(encoded); err != nil {
		return ""
	}
	return digest
}

func isDockerfile(p string) bool {
	name := path.Base(p)
	if name == "Dockerfile" || strings.HasSuffix(name, ".Dockerfile") {
		return true
	}
	return strings.HasPrefix(name, "Dockerfile.") &&
		!strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".dockerignore") &&
		!strings.HasSuffix(name, ".txt")
}
