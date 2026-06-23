package config

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

type Reloader struct {
	cfgPath      string
	pollInterval time.Duration
	lastModTime  atomic.Int64
	lastHash     atomic.Value
}

func NewReloader(cfgPath string, pollIntervalSec int) *Reloader {
	r := &Reloader{
		cfgPath:      cfgPath,
		pollInterval: time.Duration(pollIntervalSec) * time.Second,
	}
	info, err := os.Stat(cfgPath)
	if err == nil {
		r.lastModTime.Store(info.ModTime().UnixNano())
	}
	data, err := os.ReadFile(cfgPath)
	if err == nil {
		sum := sha256.Sum256(data)
		r.lastHash.Store(hex.EncodeToString(sum[:]))
	}
	return r
}

func (r *Reloader) CheckAndReload(current *Config) (*Config, bool) {
	info, err := os.Stat(r.cfgPath)
	if err != nil {
		slog.Error("config reload: stat failed", "error", err, "path", r.cfgPath)
		return nil, false
	}

	modTime := info.ModTime().UnixNano()
	if modTime == r.lastModTime.Load() {
		return nil, false
	}

	data, err := os.ReadFile(r.cfgPath)
	if err != nil {
		slog.Error("config reload: read failed", "error", err)
		return nil, false
	}

	sum := sha256.Sum256(data)
	newHash := hex.EncodeToString(sum[:])
	oldHash := ""
	if v := r.lastHash.Load(); v != nil {
		oldHash = v.(string)
	}
	if newHash == oldHash {
		r.lastModTime.Store(modTime)
		return nil, false
	}

	newCfg, err := Load(r.cfgPath)
	if err != nil {
		slog.Error("config reload: validation failed, keeping old config", "error", err)
		return nil, false
	}

	r.lastModTime.Store(modTime)
	r.lastHash.Store(newHash)
	slog.Info("config reloaded", "path", r.cfgPath, "patterns", len(newCfg.Patterns))
	return newCfg, true
}

func (r *Reloader) Start(current **Config, onReload func(*Config), stopCh <-chan struct{}) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			newCfg, changed := r.CheckAndReload(*current)
			if changed && newCfg != nil {
				onReload(newCfg)
			}
		}
	}
}
