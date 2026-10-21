package assets

import (
	"maps"

	"gopkg.in/yaml.v3"
)

func ciEnvironment(node *yaml.Node, location string) (map[string]string, error) {
	if node == nil {
		return make(map[string]string), nil
	}
	fields, err := ciMapping(node, location)
	if err != nil {
		return nil, err
	}
	env := make(map[string]string, len(fields))
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i].Value
		value, err := ciScalar(node.Content[i+1], location+"."+key)
		if err != nil {
			return nil, err
		}
		env[key] = value
	}
	return env, nil
}

// Each step gets a fresh map, so a caller that edits one step's environment
// never reaches the job's or another step's.
func ciMergedEnvironment(job, step map[string]string) map[string]string {
	env := make(map[string]string, len(job)+len(step))
	maps.Copy(env, job)
	maps.Copy(env, step)
	return env
}
