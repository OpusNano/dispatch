package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMissingAllLevelsRejected(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "levels:\n", "levels:\n  placeholder: {}\n", 1)
	raw = strings.Replace(raw, "  easy:\n", "  # easy:\n", 1)
	raw = strings.Replace(raw, "  medium:\n", "  # medium:\n", 1)
	raw = strings.Replace(raw, "  hard:\n", "  # hard:\n", 1)
	raw = strings.Replace(raw, "  critical:\n", "  # critical:\n", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for missing all levels")
	}
}

func TestThresholdsOutOfOrderRejected(t *testing.T) {
	tests := []struct {
		name    string
		find    string
		replace string
	}{
		{"easy_max <= easy", "easy_max: 20", "easy_max: 0"},
		{"medium_max <= easy_max", "medium_max: 45", "medium_max: 20"},
		{"hard_max <= medium_max", "hard_max: 70", "hard_max: 45"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := strings.Replace(defaultConfigYAML, tt.find, tt.replace, 1)
			_, err := loadFromBytes([]byte(raw))
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
		})
	}
}

func TestRequiresCycleDoesNotPanic(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		"requires: [failed_tests, stack_trace]",
		"requires: [stuck_agent]", 1)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on requires cycle: %v", r)
		}
	}()
	cfg, err := loadFromBytes([]byte(raw))
	if err != nil {
		t.Fatalf("cyclic requires should not cause validation error (rule just never fires): %v", err)
	}
	_ = cfg
}

func TestRequiresSelfReferenceDoesNotPanic(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		"requires: [failed_tests, stack_trace]",
		"requires: [stuck_agent, stuck_agent]", 1)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on self-referencing requires: %v", r)
		}
	}()
	_, err := loadFromBytes([]byte(raw))
	if err != nil {
		t.Fatalf("self-referencing requires should not cause validation error: %v", err)
	}
}

func TestUnknownYAMLFieldsIgnored(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		`version: ""`,
		`version: ""
unknown_top_level_field: "hello"`, 1)
	cfg, err := loadFromBytes([]byte(raw))
	if err != nil {
		t.Fatalf("unknown top-level YAML field should be ignored: %v", err)
	}
	if cfg.Server.Listen != ":18087" {
		t.Errorf("listen = %s, want :18087", cfg.Server.Listen)
	}
}

func TestUnknownLevelRejected(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		"  critical:\n    model:",
		"  critical:\n    model:", 1)
	raw = raw + "\n  extra_level:\n    model: \"test/model\"\n"
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for unknown level")
	}
}

func TestConfigWithPinnedModelIDs(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		`model: "deepseek/deepseek-v4-flash"`,
		`model: "anthropic/claude-3.5-sonnet"`, 1)
	raw = strings.Replace(raw,
		`model: "deepseek/deepseek-v4-pro"`,
		`model: "openai/gpt-4o"`, 1)
	raw = strings.Replace(raw,
		`model: "z-ai/glm-5.2"`,
		`model: "google/gemini-2.0-flash"`, 1)
	cfg, err := loadFromBytes([]byte(raw))
	if err != nil {
		t.Fatalf("pinned model IDs should be accepted: %v", err)
	}
	if cfg.Levels["easy"].Model != "anthropic/claude-3.5-sonnet" {
		t.Errorf("easy model = %s", cfg.Levels["easy"].Model)
	}
	if cfg.Levels["hard"].Model != "openai/gpt-4o" {
		t.Errorf("hard model = %s", cfg.Levels["hard"].Model)
	}
	if cfg.Levels["critical"].Model != "google/gemini-2.0-flash" {
		t.Errorf("critical model = %s", cfg.Levels["critical"].Model)
	}
}

func TestPatternWithEmptyRegexAndNoMatchFlagsRejected(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		`regex: 'write|implement|create|build|generate|code|program|script|function|class|module|handler|endpoint|api|service|controller|refactor|debug|fix|optimize|redesign|restructure|migrate|architecture'`,
		`regex: ""`, 1)
	raw = strings.Replace(raw,
		`dimension: complexity
    weight: 22`,
		`dimension: complexity
    weight: 22`, 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for pattern with empty regex and no match flags")
	}
}

func TestNegativeScoringCapsRejected(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		"complexity_cap: 40",
		"complexity_cap: -1", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for negative complexity cap")
	}
}

func TestRequiresNotDanglingRefRejected(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		"requires_not: [database_topic, production_topic]",
		"requires_not: [nonexistent_pattern]", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for dangling requires_not reference")
	}
}

func TestEnsureConfigDirDoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()

	customYAML := "# custom config\n" + defaultConfigYAML
	yamlPath := filepath.Join(dir, "router.yaml")
	if err := os.WriteFile(yamlPath, []byte(customYAML), 0644); err != nil {
		t.Fatal(err)
	}

	customMD := "# custom DISPATCH.md\ncustom content"
	mdPath := filepath.Join(dir, "DISPATCH.md")
	if err := os.WriteFile(mdPath, []byte(customMD), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureConfigDir(dir); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	content, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# custom config") {
		t.Error("EnsureConfigDir overwrote existing router.yaml")
	}

	content, err = os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "# custom DISPATCH.md") {
		t.Error("EnsureConfigDir overwrote existing DISPATCH.md")
	}
}

func TestEmptyProviderConfigProducesNoProvider(t *testing.T) {
	cfg, err := loadFromBytes([]byte(defaultConfigYAML))
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []string{"easy", "medium", "hard", "critical"} {
		lcfg := cfg.Levels[level]
		if len(lcfg.Provider.Order) != 0 {
			t.Errorf("level %s: default order should be empty, got %v", level, lcfg.Provider.Order)
		}
	}
}

func TestDuplicateIDErrorMessage(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "id: tool_call_present", "id: tool_call_present\n  - id: tool_call_present", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error should mention duplicate, got: %v", err)
	}
}

func TestUnknownDimensionErrorMessage(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "dimension: complexity", "dimension: bogus", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "dimension") {
		t.Errorf("error should mention dimension, got: %v", err)
	}
}

func TestConfigYAMLRoundTrip(t *testing.T) {
	cfg1, err := loadFromBytes([]byte(defaultConfigYAML))
	if err != nil {
		t.Fatal(err)
	}
	out, err := yaml.Marshal(cfg1)
	if err != nil {
		t.Fatal(err)
	}
	cfg2 := &Config{}
	if err := yaml.Unmarshal(out, cfg2); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}
	if err := cfg2.CompileAndValidate(); err != nil {
		t.Fatalf("round-trip validation failed: %v", err)
	}
}
