package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type exemplarsFile struct {
	Exemplars []ExemplarDef `yaml:"exemplars"`
}

func LoadExemplars(path string) ([]ExemplarDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("exemplars file %s: %w", path, err)
	}
	var f exemplarsFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("exemplars file %s: parse error: %w", path, err)
	}
	if len(f.Exemplars) == 0 {
		return nil, fmt.Errorf("exemplars file %s: no exemplars found", path)
	}
	return f.Exemplars, nil
}

func GenerateDefaultExemplarsFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte(defaultExemplarsYAML), 0644)
}

func MergeExemplars(builtIn []ExemplarDef, external []ExemplarDef, mode string) ([]ExemplarDef, error) {
	switch mode {
	case "disabled":
		return nil, nil
	case "replace":
		if len(external) == 0 {
			return nil, fmt.Errorf("exemplars merge_mode=replace but no external exemplars loaded")
		}
		return external, nil
	case "extend":
		seen := make(map[string]bool)
		for _, e := range builtIn {
			key := exemplarKey(e)
			seen[key] = true
		}
		result := make([]ExemplarDef, len(builtIn))
		copy(result, builtIn)
		for _, e := range external {
			if !seen[exemplarKey(e)] {
				result = append(result, e)
				seen[exemplarKey(e)] = true
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unknown exemplars merge_mode: %s (must be extend|replace|disabled)", mode)
	}
}

func exemplarKey(e ExemplarDef) string {
	neg := ""
	if e.Negative {
		neg = ":negative"
	}
	return fmt.Sprintf("%s:%s%s", e.Text, e.Level, neg)
}
