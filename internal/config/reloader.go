package config

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"sync/atomic"
	"time"
)

const reloadLogSuppressInterval = 5 * time.Minute

type Reloader struct {
	cfgPath      string
	pollInterval time.Duration
	lastModTime  atomic.Int64
	lastHash     atomic.Value
	State        *ReloadState

	lastLoggedError string
	lastLogTime     time.Time
}

func NewReloader(cfgPath string, pollIntervalSec int, state *ReloadState) *Reloader {
	r := &Reloader{
		cfgPath:      cfgPath,
		pollInterval: time.Duration(pollIntervalSec) * time.Second,
		State:        state,
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

func (r *Reloader) CheckAndReload() (*Config, bool) {
	info, err := os.Stat(r.cfgPath)
	if err != nil {
		r.recordAndLogFailure("config reload: stat failed: " + err.Error())
		return nil, false
	}

	modTime := info.ModTime().UnixNano()
	if modTime == r.lastModTime.Load() {
		return nil, false
	}

	data, err := os.ReadFile(r.cfgPath)
	if err != nil {
		r.recordAndLogFailure("config reload: read failed: " + err.Error())
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
		r.lastModTime.Store(modTime)
		r.lastHash.Store(newHash)
		r.recordAndLogFailure(err.Error())
		return nil, false
	}

	r.lastModTime.Store(modTime)
	r.lastHash.Store(newHash)

	r.State.RecordSuccess()

	consecutive := r.State.consecutiveFailureCount.Swap(0)
	if consecutive > 0 {
		failureDuration := time.Duration(0)
		if firstSeen := r.State.failureFirstSeenUnix.Swap(0); firstSeen > 0 {
			failureDuration = time.Since(time.Unix(firstSeen, 0))
		}
		slog.Info("config reload: recovered",
			"path", r.cfgPath,
			"patterns", len(newCfg.Patterns),
			"failed_attempts", consecutive,
			"failure_duration_seconds", int64(failureDuration.Seconds()),
		)
	} else {
		slog.Info("config reloaded", "path", r.cfgPath, "patterns", len(newCfg.Patterns))
	}

	r.lastLoggedError = ""
	return newCfg, true
}

func (r *Reloader) recordAndLogFailure(errStr string) {
	r.State.RecordFailure(errStr)

	shouldLog := false
	if errStr != r.lastLoggedError {
		shouldLog = true
	} else if time.Since(r.lastLogTime) >= reloadLogSuppressInterval {
		shouldLog = true
	}

	if shouldLog {
		r.lastLoggedError = errStr
		r.lastLogTime = time.Now()
		if r.State.ActiveConfigState() == "degraded_using_last_valid" && r.State.consecutiveFailureCount.Load() > 1 {
			slog.Error("config reload: validation failed, keeping old config (repeated)",
				"consecutive_failures", r.State.consecutiveFailureCount.Load(),
				"error", errStr,
			)
		} else {
			slog.Error("config reload: validation failed, keeping old config", "error", errStr)
		}
	}
}

func (r *Reloader) Start(onReload func(*Config), stopCh <-chan struct{}) {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			newCfg, changed := r.CheckAndReload()
			if changed && newCfg != nil {
				onReload(newCfg)
			}
		}
	}
}
