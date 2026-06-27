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

	if _, err := Load(cfgPath); err != nil {
		t.Fatal(err)
	}

	reloader := NewReloader(cfgPath, 1, NewReloadState())

	time.Sleep(100 * time.Millisecond)

	_, changed := reloader.CheckAndReload()
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

	newCfg, changed := reloader.CheckAndReload()
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

	if _, err := Load(cfgPath); err != nil {
		t.Fatal(err)
	}

	reloader := NewReloader(cfgPath, 1, NewReloadState())

	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: :::"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cfgPath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	newCfg, changed := reloader.CheckAndReload()
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

func TestReloadStateStartsOk(t *testing.T) {
	rs := NewReloadState()
	if rs.ActiveConfigState() != "ok" {
		t.Errorf("initial state should be ok, got %s", rs.ActiveConfigState())
	}
	snap := rs.Snapshot()
	if snap.ActiveConfigState != "ok" {
		t.Errorf("snapshot state should be ok, got %s", snap.ActiveConfigState)
	}
}

func TestReloadStateRecordFailure(t *testing.T) {
	rs := NewReloadState()
	rs.RecordFailure("yaml: unmarshal errors: line 42: duplicate key")

	if rs.ActiveConfigState() != "degraded_using_last_valid" {
		t.Errorf("state should be degraded after failure, got %s", rs.ActiveConfigState())
	}

	snap := rs.Snapshot()
	if snap.ActiveConfigState != "degraded_using_last_valid" {
		t.Error("snapshot should show degraded")
	}
	if snap.ConfigReloadFailureCount != 1 {
		t.Errorf("failure count = %d, want 1", snap.ConfigReloadFailureCount)
	}
	if snap.LastConfigReloadFailureUnix == 0 {
		t.Error("last_failure_unix should be non-zero")
	}
	if snap.ConfigReloadConsecutiveFailures != 1 {
		t.Errorf("consecutive failures = %d, want 1", snap.ConfigReloadConsecutiveFailures)
	}
	if snap.ConfigReloadFailureFirstSeenUnix == 0 {
		t.Error("failure_first_seen_unix should be non-zero")
	}
	if !containsStr(snap.LastConfigReloadErrorTruncated, "duplicate key") {
		t.Errorf("error should contain 'duplicate key', got: %s", snap.LastConfigReloadErrorTruncated)
	}
}

func TestReloadStateRecordSuccess(t *testing.T) {
	rs := NewReloadState()
	rs.RecordFailure("bad config")
	rs.RecordSuccess()

	if rs.ActiveConfigState() != "ok" {
		t.Errorf("state should be ok after recovery, got %s", rs.ActiveConfigState())
	}

	snap := rs.Snapshot()
	if snap.ActiveConfigState != "ok" {
		t.Error("snapshot should show ok after recovery")
	}
	if snap.ConfigReloadSuccessCount != 1 {
		t.Errorf("success count = %d, want 1", snap.ConfigReloadSuccessCount)
	}
	if snap.LastConfigReloadSuccessUnix == 0 {
		t.Error("last_success_unix should be non-zero")
	}
	if snap.ConfigReloadConsecutiveFailures != 0 {
		t.Errorf("consecutive failures should be 0 after recovery, got %d", snap.ConfigReloadConsecutiveFailures)
	}
}

func TestReloadStateErrorTruncation(t *testing.T) {
	rs := NewReloadState()
	longErr := ""
	for i := 0; i < 600; i++ {
		longErr += "x"
	}
	rs.RecordFailure(longErr)

	snap := rs.Snapshot()
	if len(snap.LastConfigReloadErrorTruncated) > 500 {
		t.Errorf("error truncated length = %d, want <= 500", len(snap.LastConfigReloadErrorTruncated))
	}
}

func TestReloadFailureUpdatesState(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "router.yaml")

	if err := os.WriteFile(cfgPath, []byte(defaultConfigYAML), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(cfgPath); err != nil {
		t.Fatal(err)
	}

	rs := NewReloadState()
	reloader := NewReloader(cfgPath, 1, rs)

	time.Sleep(100 * time.Millisecond)

	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: :::"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cfgPath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	newCfg, changed := reloader.CheckAndReload()
	if changed || newCfg != nil {
		t.Fatal("should keep old config on bad yaml")
	}

	if rs.ActiveConfigState() != "degraded_using_last_valid" {
		t.Errorf("state should be degraded, got %s", rs.ActiveConfigState())
	}
	if rs.Snapshot().ConfigReloadFailureCount == 0 {
		t.Error("failure count should be non-zero")
	}
}

func TestReloadRecoveryClearsDegraded(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "router.yaml")

	if err := os.WriteFile(cfgPath, []byte(defaultConfigYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfgPath); err != nil {
		t.Fatal(err)
	}

	rs := NewReloadState()
	reloader := NewReloader(cfgPath, 1, rs)

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(cfgPath, []byte("invalid: yaml: :::"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cfgPath, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	reloader.CheckAndReload()

	if rs.ActiveConfigState() != "degraded_using_last_valid" {
		t.Fatalf("should be degraded after bad config: %s", rs.ActiveConfigState())
	}

	if err := os.WriteFile(cfgPath, []byte(defaultConfigYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(cfgPath, time.Now().Add(time.Hour), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	reloader.CheckAndReload()

	if rs.ActiveConfigState() != "ok" {
		t.Errorf("should be ok after recovery, got %s", rs.ActiveConfigState())
	}
	if rs.Snapshot().ConfigReloadSuccessCount == 0 {
		t.Error("success count should be non-zero after recovery")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
