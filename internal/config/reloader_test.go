package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReloaderDetectsChange(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "router.yaml")

	if err := os.WriteFile(cfgPath, []byte(defaultConfigYAML), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	reloader := NewReloader(cfgPath, 1)

	time.Sleep(100 * time.Millisecond)

	_, changed := reloader.CheckAndReload(loaded)
	if changed {
		t.Fatal("should not detect change without modification")
	}

	modified := defaultConfigYAML + "\n# modified\n"
	if err := os.WriteFile(cfgPath, []byte(modified), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cfgPath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	newCfg, changed := reloader.CheckAndReload(loaded)
	if !changed || newCfg == nil {
		t.Fatal("should detect content change")
	}
}

func TestReloaderKeepsOldOnBadConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "router.yaml")

	initialCfg, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteTestConfig(cfgPath, initialCfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	reloader := NewReloader(cfgPath, 1)

	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: :::"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cfgPath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	newCfg, changed := reloader.CheckAndReload(loaded)
	if changed || newCfg != nil {
		t.Fatal("should keep old config on bad yaml")
	}
}

func WriteTestConfig(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		return nil
	}
	defaultCfg, err := DefaultConfig()
	if err != nil {
		return err
	}
	_ = defaultCfg
	return os.WriteFile(path, []byte(defaultConfigYAML), 0644)
}
