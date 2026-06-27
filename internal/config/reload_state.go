package config

import (
	"sync"
	"sync/atomic"
	"time"
)

type ReloadState struct {
	activeConfigState       string
	stateMu                 sync.RWMutex
	successCount            atomic.Int64
	failureCount            atomic.Int64
	lastSuccessUnix         atomic.Int64
	lastFailureUnix         atomic.Int64
	lastErrorTruncated      string
	lastErrorMu             sync.Mutex
	consecutiveFailureCount atomic.Int64
	failureFirstSeenUnix    atomic.Int64
}

func NewReloadState() *ReloadState {
	s := &ReloadState{}
	s.activeConfigState = "ok"
	return s
}

func (s *ReloadState) ActiveConfigState() string {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.activeConfigState
}

func (s *ReloadState) RecordSuccess() {
	s.stateMu.Lock()
	s.activeConfigState = "ok"
	s.stateMu.Unlock()

	s.successCount.Add(1)
	s.lastSuccessUnix.Store(time.Now().Unix())
	s.consecutiveFailureCount.Store(0)
	s.failureFirstSeenUnix.Store(0)

	s.lastErrorMu.Lock()
	s.lastErrorTruncated = ""
	s.lastErrorMu.Unlock()
}

func (s *ReloadState) RecordFailure(err string) {
	if len(err) > 500 {
		err = err[:500]
	}

	s.stateMu.Lock()
	s.activeConfigState = "degraded_using_last_valid"
	s.stateMu.Unlock()

	s.failureCount.Add(1)
	now := time.Now().Unix()
	s.lastFailureUnix.Store(now)

	prev := s.consecutiveFailureCount.Add(1)
	if prev == 1 {
		s.failureFirstSeenUnix.Store(now)
	}

	s.lastErrorMu.Lock()
	s.lastErrorTruncated = err
	s.lastErrorMu.Unlock()
}

type ReloadStateSnapshot struct {
	ActiveConfigState                string `json:"active_config_state"`
	ConfigReloadSuccessCount         int64  `json:"config_reload_success_count"`
	ConfigReloadFailureCount         int64  `json:"config_reload_failure_count"`
	LastConfigReloadSuccessUnix      int64  `json:"last_config_reload_success_unix"`
	LastConfigReloadFailureUnix      int64  `json:"last_config_reload_failure_unix"`
	LastConfigReloadErrorTruncated   string `json:"last_config_reload_error_truncated,omitempty"`
	ConfigReloadConsecutiveFailures  int64  `json:"config_reload_consecutive_failure_count"`
	ConfigReloadFailureFirstSeenUnix int64  `json:"config_reload_failure_first_seen_unix"`
}

func (s *ReloadState) Snapshot() ReloadStateSnapshot {
	s.lastErrorMu.Lock()
	errCopy := s.lastErrorTruncated
	s.lastErrorMu.Unlock()

	return ReloadStateSnapshot{
		ActiveConfigState:                s.ActiveConfigState(),
		ConfigReloadSuccessCount:         s.successCount.Load(),
		ConfigReloadFailureCount:         s.failureCount.Load(),
		LastConfigReloadSuccessUnix:      s.lastSuccessUnix.Load(),
		LastConfigReloadFailureUnix:      s.lastFailureUnix.Load(),
		LastConfigReloadErrorTruncated:   errCopy,
		ConfigReloadConsecutiveFailures:  s.consecutiveFailureCount.Load(),
		ConfigReloadFailureFirstSeenUnix: s.failureFirstSeenUnix.Load(),
	}
}
