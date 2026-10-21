package policyfiles

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/cristianargotti/golaw/internal/model"
)

const imageAllowedTags = "allowed_tags"

type imageDigest struct{}

// ImageDigest requires immutable external bases while preserving declared tag exceptions.
func ImageDigest() model.Check { return imageDigest{} }

func (imageDigest) Spec() model.Spec {
	definition := spec("image-digest", ruleSupply, "image")
	definition.Options = model.OptionSchema{{
		Name: imageAllowedTags, Kind: model.OptionStrings,
		Tighten: model.TightenShrink, Default: model.StringsValue(),
	}}
	return definition
}

func (imageDigest) Run(_ context.Context, in model.Input) (model.Result, error) {
	result := model.Result{Status: model.Measured()}
	patterns, _ := in.Options.Strings(imageAllowedTags)
	count := 0
	for _, file := range model.Select(in.Snapshot, isDockerfile) {
		if file.Class == model.ClassFixture {
			continue
		}
		count++
		document, err := readImageFile(in, file.Path)
		if err != nil {
			return answer(file.Path, err), nil
		}
		findings, err := imageFindings(document, patterns)
		if err != nil {
			return answer(file.Path, err), nil
		}
		result.Findings = append(result.Findings, findings...)
	}
	result.Scope = strconv.Itoa(count) + " Dockerfiles"
	return result, nil
}

func readImageFile(in model.Input, file string) (dockerfile, error) {
	data, found, err := read(in, file)
	if err != nil {
		return dockerfile{}, err
	}
	if !found {
		return dockerfile{}, fmt.Errorf("%s: %w", file, model.ErrNotInSnapshot)
	}
	return readDockerfile(file, data)
}

func imageFindings(document dockerfile, patterns []string) ([]model.Finding, error) {
	aliases := make(map[string]bool)
	var findings []model.Finding
	for _, stage := range document.stages {
		if stage.digest == "" && !strings.EqualFold(stage.from, "scratch") && !aliases[strings.ToLower(stage.from)] {
			allowed, err := imageTagAllowed(stage.from, patterns)
			if err != nil {
				return nil, err
			}
			if !allowed {
				message := "pin the base image " + stage.from + " in " + document.path +
					" to a complete digest: write the reference with @sha256: and its 64 hexadecimal digits"
				findings = append(findings, finding(document.path, stage.line, "image-digest-missing", message))
			}
		}
		if stage.name != "" {
			aliases[strings.ToLower(stage.name)] = true
		}
	}
	return findings, nil
}

func imageTagAllowed(reference string, patterns []string) (bool, error) {
	name, _, _ := strings.Cut(reference, "@")
	if colon := strings.LastIndexByte(name, ':'); colon > strings.LastIndexByte(name, '/') {
		name = name[:colon]
	}
	for _, pattern := range patterns {
		matched, err := path.Match(pattern, name)
		if err != nil {
			return false, parseError("golaw.yml allowed_tags pattern "+strconv.Quote(pattern), err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
