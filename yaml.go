package localengine

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// loadTree parses YAML data into a nested map.
func loadTree(data []byte) (map[string]any, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal yaml: %w", err)
	}
	if raw == nil {
		return make(map[string]any), nil
	}
	return raw, nil
}
