package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func loadFromBytes(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if err := cfg.CompileAndValidate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func TestLoadDefaultConfig(t *testing.T) {
	cfg, err := loadFromBytes([]byte(defaultConfigYAML))
	if err != nil {
		t.Fatalf("default config should load: %v", err)
	}
	if cfg.Server.MaxBodySize != 26214400 {
		t.Errorf("max_body_size = %d, want 26214400", cfg.Server.MaxBodySize)
	}
	if cfg.Thresholds.EasyMax <= cfg.Thresholds.Easy {
		t.Error("easy_max should be > easy")
	}
	for _, level := range []string{"easy", "medium", "hard", "critical"} {
		if _, ok := cfg.Levels[level]; !ok {
			t.Errorf("missing level: %s", level)
		}
	}
	if cfg.Levels["easy"].Use != "deepseek_flash" {
		t.Errorf("easy level use = %s, want deepseek_flash", cfg.Levels["easy"].Use)
	}
	rm, _ := cfg.ResolveLevel("easy")
	if rm.Model != "deepseek/deepseek-v4-flash" {
		t.Errorf("easy model = %s", rm.Model)
	}
	if cfg.ModelProfiles["deepseek_flash"].Provider.DataCollection != "deny" {
		t.Errorf("easy data_collection = %s", cfg.ModelProfiles["deepseek_flash"].Provider.DataCollection)
	}
	if len(cfg.CompiledPatterns()) == 0 {
		t.Error("patterns should be compiled")
	}
}

func TestMissingLevel(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "easy:", "easymissing:", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for missing easy level")
	}
}

func TestMissingModel(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "id: \"deepseek/deepseek-v4-flash\"", "id: \"\"", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}

func TestInvalidThresholds(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "easy_max: 20", "easy_max: 0", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for easy_max <= easy")
	}
}

func TestInvalidDimension(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "dimension: complexity", "dimension: invalid", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for invalid dimension")
	}
}

func TestDuplicatePatternID(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "id: tool_call_present", "id: tool_call_present\n  - id: tool_call_present", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for duplicate id")
	}
}

func TestBadRegex(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "diff --git'", "diff --git(invalid'", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "invalid regex") && !strings.Contains(err.Error(), "error parsing regexp") {
		t.Errorf("expected regex error, got: %v", err)
	}
}

func TestDanglingRequires(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML,
		"requires: [failed_tests, stack_trace]",
		"requires: [failed_tests, nonexistent_rule]", 1)
	_, err := loadFromBytes([]byte(raw))
	if err == nil {
		t.Fatal("expected error for dangling requires ref")
	}
}

func TestFirstRunGeneration(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureConfigDir(dir); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	yamlPath := filepath.Join(dir, "router.yaml")
	mdPath := filepath.Join(dir, "DISPATCH.md")

	if _, err := os.Stat(yamlPath); os.IsNotExist(err) {
		t.Fatal("router.yaml not created")
	}
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Fatal("DISPATCH.md not created")
	}

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("generated config should be valid: %v", err)
	}
	_ = cfg

	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mdContent), "Instructions for LLMs") {
		t.Error("ROUTER.md missing LLM editing instructions")
	}
}

func TestCompiledPatternsPresent(t *testing.T) {
	cfg, err := loadFromBytes([]byte(defaultConfigYAML))
	if err != nil {
		t.Fatal(err)
	}
	patterns := cfg.CompiledPatterns()
	if len(patterns) == 0 {
		t.Fatal("no compiled patterns")
	}

	toolRule, ok := patterns["tool_call_present"]
	if !ok {
		t.Fatal("tool_call_present not found")
	}
	if !toolRule.Pattern.MatchTools {
		t.Error("tool_call_present should have match_tools=true")
	}
	if toolRule.Pattern.Dimension != "complexity" {
		t.Errorf("tool_call_present dimension = %s, want complexity", toolRule.Pattern.Dimension)
	}

	diffRule, ok := patterns["diff_patch"]
	if !ok {
		t.Fatal("diff_patch not found")
	}
	if diffRule.Regex == nil {
		t.Fatal("diff_patch regex not compiled")
	}
	if !diffRule.Regex.MatchString("@@ -1,5 +1,6 @@") {
		t.Error("diff_patch regex should match diff header")
	}
}

func TestProviderMergeField(t *testing.T) {
	raw := strings.Replace(defaultConfigYAML, "data_collection: \"deny\"", "data_collection: \"allow\"", 1)
	cfg, err := loadFromBytes([]byte(raw))
	if err != nil {
		t.Fatalf("valid data_collection should work: %v", err)
	}
	if cfg.ModelProfiles["deepseek_flash"].Provider.DataCollection != "allow" {
		t.Errorf("data_collection = %s, want allow", cfg.ModelProfiles["deepseek_flash"].Provider.DataCollection)
	}
}
