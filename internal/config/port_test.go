package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigListenPort(t *testing.T) {
	cfg, err := loadFromBytes([]byte(defaultConfigYAML))
	if err != nil {
		t.Fatalf("default config should load: %v", err)
	}
	if cfg.Server.Listen != ":18087" {
		t.Errorf("default listen = %s, want :18087", cfg.Server.Listen)
	}
}

func TestGeneratedConfigListenPort(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureConfigDir(dir); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}
	yamlPath := filepath.Join(dir, "router.yaml")
	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("generated config should load: %v", err)
	}
	if cfg.Server.Listen != ":18087" {
		t.Errorf("generated config listen = %s, want :18087", cfg.Server.Listen)
	}
}

func TestGeneratedDocsMentionPort18087(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureConfigDir(dir); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}
	mdPath := filepath.Join(dir, "DISPATCH.md")
	mdContent, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mdContent), ":18087") {
		t.Error("DISPATCH.md should mention :18087")
	}
}

func TestNoStalePort8080InDefaults(t *testing.T) {
	if strings.Contains(defaultConfigYAML, "8080") {
		t.Error("default config YAML contains stale 8080 reference")
	}
	if strings.Contains(defaultROUTERmd, "8080") {
		t.Error("default DISPATCH.md contains stale 8080 reference")
	}
}
