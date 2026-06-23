package classifier

import (
	"sync"
	"time"

	"dispatch/internal/config"
)

type sessionEntry struct {
	FailureCount         int
	ToolErrorCount       int
	RepeatedFailureCount int
	LastLevel            string
	LastRisk             Risk
	LastTimestamp        time.Time
}

type sessionStore struct {
	mu         sync.RWMutex
	entries    map[string]*sessionEntry
	maxEntries int
	ttl        time.Duration
}

var sessions *sessionStore

func InitSessions(cfg *config.Config) {
	if cfg.Intelligence == nil || !cfg.Intelligence.Enabled || !cfg.Intelligence.Session.Enabled {
		sessions = nil
		return
	}
	sessions = &sessionStore{
		entries:    make(map[string]*sessionEntry),
		maxEntries: cfg.Intelligence.Session.MaxEntries,
		ttl:        time.Duration(cfg.Intelligence.Session.TTLMinutes) * time.Minute,
	}
}

type SessionInfo struct {
	Enabled              bool   `json:"enabled"`
	SessionID            string `json:"session_id"`
	Tracked              bool   `json:"tracked"`
	FailureCount         int    `json:"failure_count,omitempty"`
	ToolErrorCount       int    `json:"tool_error_count,omitempty"`
	RepeatedFailureCount int    `json:"repeated_failure_count,omitempty"`
	Escalated            bool   `json:"escalated"`
	LastLevel            string `json:"last_level,omitempty"`
	LastRisk             Risk   `json:"last_risk,omitempty"`

	TaskKey                 string `json:"task_key,omitempty"`
	TaskKeySource           string `json:"task_key_source,omitempty"`
	UsedPreviousState       bool   `json:"used_previous_state"`
	UsedCurrentFrame        bool   `json:"used_current_frame"`
	EscalationReason        string `json:"escalation_reason,omitempty"`
	ProspectiveFailureCount int    `json:"prospective_failure_count,omitempty"`
}

func sessionTaskKey(sessionID, taskKey string) string {
	if sessionID == "" {
		return ""
	}
	if taskKey == "" {
		return sessionID
	}
	return sessionID + ":" + taskKey
}

func CheckSessionEscalation(sessionID, taskKey string, facts Facts, cfg *config.Config) (string, *SessionInfo, bool) {
	info := &SessionInfo{
		Enabled: cfg.Intelligence != nil && cfg.Intelligence.Session.Enabled,
		TaskKey: taskKey,
	}
	if cfg.Intelligence == nil || !cfg.Intelligence.Session.Enabled {
		return "", info, false
	}

	if sessions == nil {
		InitSessions(cfg)
	}
	if sessions == nil {
		return "", info, false
	}

	if sessionID == "" {
		return "", info, false
	}
	info.SessionID = sessionID
	info.Tracked = true

	compositeKey := sessionTaskKey(sessionID, taskKey)
	sessionCfg := cfg.Intelligence.Session

	sessions.mu.Lock()
	entry, exists := sessions.entries[compositeKey]
	if !exists {
		entry = &sessionEntry{LastTimestamp: time.Now()}
		if len(sessions.entries) >= sessions.maxEntries {
			for k := range sessions.entries {
				delete(sessions.entries, k)
				break
			}
		}
		sessions.entries[compositeKey] = entry
	}
	sessions.mu.Unlock()

	sessions.mu.RLock()
	totalFromPrevious := entry.FailureCount + entry.RepeatedFailureCount + entry.ToolErrorCount
	info.FailureCount = entry.FailureCount
	info.ToolErrorCount = entry.ToolErrorCount
	info.RepeatedFailureCount = entry.RepeatedFailureCount
	info.LastLevel = entry.LastLevel
	info.LastRisk = entry.LastRisk
	sessions.mu.RUnlock()

	if time.Since(entry.LastTimestamp) > sessions.ttl {
		sessions.mu.Lock()
		delete(sessions.entries, compositeKey)
		sessions.mu.Unlock()
		info.Tracked = false
		return "", info, false
	}

	currentFrameFailures := 0
	if hasEvidence(facts.Evidence, EvidenceStackTrace) {
		currentFrameFailures++
	}
	if hasEvidence(facts.Evidence, EvidenceTestFailure) {
		currentFrameFailures++
	}
	if hasEvidence(facts.Evidence, EvidenceCompileError) {
		currentFrameFailures++
	}
	if hasEvidence(facts.Evidence, EvidenceToolError) {
		currentFrameFailures++
	}
	if hasEvidence(facts.Evidence, EvidencePatch) {
		currentFrameFailures++
	}
	if hasEvidence(facts.Evidence, EvidenceRepeatedFailure) {
		currentFrameFailures++
	}

	prospectiveTotal := totalFromPrevious + currentFrameFailures
	info.ProspectiveFailureCount = prospectiveTotal

	if totalFromPrevious > 0 {
		info.UsedPreviousState = true
	}
	if currentFrameFailures > 0 {
		info.UsedCurrentFrame = true
	}

	var escalatedLevel string
	if prospectiveTotal >= sessionCfg.Escalation.CriticalAfterRepeatedFailures {
		escalatedLevel = "critical"
		info.Escalated = true
		info.EscalationReason = "previous_state+current_frame"
	} else if prospectiveTotal >= sessionCfg.Escalation.HardAfterRepeatedFailures {
		escalatedLevel = "hard"
		info.Escalated = true
		info.EscalationReason = "previous_state+current_frame"
	}

	return escalatedLevel, info, true
}

func UpdateSession(sessionID, taskKey string, facts Facts, cfg *config.Config) {
	if cfg.Intelligence == nil || !cfg.Intelligence.Session.Enabled {
		return
	}
	if sessions == nil || sessionID == "" {
		return
	}

	hasFailure := hasEvidence(facts.Evidence, EvidenceRepeatedFailure) ||
		hasEvidence(facts.Evidence, EvidenceToolError) ||
		hasEvidence(facts.Evidence, EvidenceTestFailure) ||
		hasEvidence(facts.Evidence, EvidenceCompileError) ||
		hasEvidence(facts.Evidence, EvidenceStackTrace) ||
		hasEvidence(facts.Evidence, EvidencePatch)

	compositeKey := sessionTaskKey(sessionID, taskKey)

	sessions.mu.Lock()
	defer sessions.mu.Unlock()

	entry, entryExists := sessions.entries[compositeKey]
	if !entryExists {
		entry = &sessionEntry{}
		sessions.entries[compositeKey] = entry
	}

	if hasFailure {
		if hasEvidence(facts.Evidence, EvidenceRepeatedFailure) {
			entry.RepeatedFailureCount++
		}
		if hasEvidence(facts.Evidence, EvidenceToolError) {
			entry.ToolErrorCount++
		}
		if hasEvidence(facts.Evidence, EvidenceTestFailure) ||
			hasEvidence(facts.Evidence, EvidenceCompileError) ||
			hasEvidence(facts.Evidence, EvidenceStackTrace) ||
			hasEvidence(facts.Evidence, EvidencePatch) {
			entry.FailureCount++
		}
	} else {
		if !hasEvidence(facts.Evidence, EvidenceToolCalls) &&
			!hasEvidence(facts.Evidence, EvidenceJSONSchema) &&
			!hasEvidence(facts.Evidence, EvidenceDiff) &&
			!hasEvidence(facts.Evidence, EvidenceLogs) {
			entry.FailureCount = maxI(0, entry.FailureCount-cfg.Intelligence.Session.Escalation.DecayPerSuccess)
			entry.RepeatedFailureCount = maxI(0, entry.RepeatedFailureCount-cfg.Intelligence.Session.Escalation.DecayPerSuccess)
		}
	}
	entry.LastTimestamp = time.Now()
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}
