package policyfiles

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/cristianargotti/golaw/internal/model"
)

type apiLocation struct {
	file, pointer string
	node          *yaml.Node
}

type apiVisit struct{ file, pointer, role string }

type apiActive struct {
	node *yaml.Node
	role string
}

type openapiReader struct {
	in        model.Input
	api       redoclyAPI
	documents map[string]*yaml.Node
	seen      map[apiVisit]bool
	active    map[apiActive]bool
	issues    []openapiIssue
	failed    string
}

func newOpenAPIReader(in model.Input, api redoclyAPI, documents map[string]*yaml.Node) *openapiReader {
	return &openapiReader{
		in: in, api: api, documents: documents,
		seen: make(map[apiVisit]bool), active: make(map[apiActive]bool),
	}
}

func (r *openapiReader) document(file string) (*yaml.Node, bool, error) {
	r.failed = file
	if node, found := r.documents[file]; found {
		return node, true, nil
	}
	data, found, err := read(r.in, file)
	if err != nil || !found {
		return nil, found, err
	}
	root, err := decodeYAML(file, data)
	if err != nil {
		return nil, true, err
	}
	r.documents[file] = root
	return root, true, nil
}

func (r *openapiReader) reference(at apiLocation, ref *yaml.Node) (apiLocation, error) {
	r.failed = at.file
	if ref.Kind != yaml.ScalarNode || ref.Tag != dependabotStringTag {
		return apiLocation{}, outside(at.file, ref.Line, "nonliteral reference")
	}
	file, pointer, err := apiReferencePath(at.file, ref)
	if err != nil {
		return apiLocation{}, err
	}
	root, found, err := r.document(file)
	if err != nil {
		return apiLocation{}, err
	}
	if !found {
		return apiLocation{}, apiMissingReference(at, ref, file)
	}
	for _, key := range []string{apiVersionKey, "swagger"} {
		if version := child(root, key); version != nil && version.Kind == yaml.ScalarNode {
			if dialectErr := openapiDialect(file, root); dialectErr != nil {
				return apiLocation{}, dialectErr
			}
		}
	}
	node, err := apiPointer(file, root, pointer)
	if errors.Is(err, model.ErrNotInSnapshot) {
		err = apiMissingReference(at, ref, file+"#"+pointer)
	}
	return apiLocation{file: file, pointer: "#" + pointer, node: node}, err
}

func apiMissingReference(at apiLocation, ref *yaml.Node, target string) error {
	return fmt.Errorf("%s:%d: $ref %s names %s, which the repository does not hold: %w",
		at.file, ref.Line, ref.Value, target, model.ErrNotInSnapshot)
}

// apiReferencePath follows only files of the repository: a remote reference
// is evidence the tree cannot hold, so it is outside the subset.
func apiReferencePath(file string, ref *yaml.Node) (string, string, error) {
	u, err := url.Parse(ref.Value)
	if err != nil {
		return "", "", parseError(file, err)
	}
	if u.IsAbs() || u.Host != "" || strings.HasPrefix(u.Path, "//") {
		return "", "", outside(file, ref.Line, "remote reference "+ref.Value)
	}
	if u.RawQuery != "" || strings.Contains(u.Path, "\\") || strings.HasPrefix(u.Path, "/") {
		return "", "", outside(file, ref.Line, "unsupported reference path "+ref.Value)
	}
	target := file
	if u.Path != "" {
		target = path.Clean(path.Join(path.Dir(file), u.Path))
	}
	if target == ".." || strings.HasPrefix(target, apiParentPrefix) {
		return "", "", outside(file, ref.Line, "reference leaves the repository: "+ref.Value)
	}
	return target, u.Fragment, nil
}

func apiPointer(file string, root *yaml.Node, pointer string) (*yaml.Node, error) {
	if pointer == "" {
		return root, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, outside(file, 1, "unsupported reference fragment #"+pointer)
	}
	node := root
	for part := range strings.SplitSeq(pointer[1:], "/") {
		key, valid := apiPointerKey(part)
		if !valid {
			return nil, outside(file, 1, "invalid JSON pointer #"+pointer)
		}
		node = apiPointerChild(resolve(node), key)
		if node == nil {
			return nil, fmt.Errorf("resolve %s#%s: %w", file, pointer, model.ErrNotInSnapshot)
		}
	}
	return resolve(node), nil
}

func apiPointerKey(part string) (string, bool) {
	for index := 0; index < len(part); index++ {
		if part[index] == '~' {
			index++
			if index == len(part) || (part[index] != '0' && part[index] != '1') {
				return "", false
			}
		}
	}
	return strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~"), true
}

func apiPointerChild(node *yaml.Node, key string) *yaml.Node {
	if node != nil && node.Kind == yaml.SequenceNode {
		index, err := strconv.Atoi(key)
		if err == nil && index >= 0 && index < len(node.Content) && strconv.Itoa(index) == key {
			return node.Content[index]
		}
		return nil
	}
	return child(node, key)
}

func apiPointerPart(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

func (at apiLocation) branch(keys string) (apiLocation, error) {
	if keys == "" {
		return at, nil
	}
	for key := range strings.SplitSeq(keys, "/") {
		if at.node != nil && at.node.Kind != yaml.MappingNode {
			return at, outside(at.file, at.node.Line, "nonmapping "+at.pointer)
		}
		at = at.descend(key)
	}
	return at, nil
}

func (r *openapiReader) walkReference(at apiLocation, role string, ref *yaml.Node) error {
	target, err := r.reference(at, ref)
	if err != nil {
		return err
	}
	if role == apiPath && apiPathOverlap(at.node, target.node) {
		return outside(at.file, ref.Line, "overlapping Path Item reference fields")
	}
	if err := r.walk(target, role); err != nil {
		return err
	}
	if role == apiPath {
		return r.followEdges(at, apiPathEdges())
	}
	if role == apiSchema && r.schemaReferenceSiblings(at.file) {
		if err := r.inspect(at, role); err != nil {
			return err
		}
		return r.followEdges(at, apiSchemaEdges())
	}
	return nil
}

func (r *openapiReader) schemaReferenceSiblings(file string) bool {
	version := child(r.documents[file], apiVersionKey)
	if version == nil || version.Kind != yaml.ScalarNode {
		version = child(r.documents[r.api.root], apiVersionKey)
	}
	return version != nil && strings.HasPrefix(version.Value, "3.1.")
}

func apiPathOverlap(local, referenced *yaml.Node) bool {
	for _, edge := range apiPathEdges() {
		if child(local, edge.key) != nil && child(referenced, edge.key) != nil {
			return true
		}
	}
	return false
}

func redoclyPath(value string) bool {
	return value != "" && !strings.ContainsAny(value, "\\:$*?[]#") && !path.IsAbs(value) &&
		path.Clean(value) != "." && path.Clean(value) != ".." && !strings.HasPrefix(path.Clean(value), apiParentPrefix)
}
