package classifier

import (
	"testing"

	"dispatch/internal/config"
)

func setupSessionTest() *config.Config {
	cfg := &config.Config{
		Intelligence: &config.IntelligenceConfig{
			Enabled: true,
			Session: config.SessionConfig{
				Enabled:    true,
				TTLMinutes: 60,
				MaxEntries: 100,
				Escalation: config.SessionEscalationConfig{
					HardAfterRepeatedFailures:     2,
					CriticalAfterRepeatedFailures: 3,
					DecayPerSuccess:               1,
				},
			},
		},
	}
	InitSessions(cfg)
	return cfg
}

func fakeFacts(evidence ...Evidence) Facts {
	return Facts{Evidence: evidence}
}

func TestProspectiveEscalationSecondFailure(t *testing.T) {
	cfg := setupSessionTest()
	sessionID := "sess-1"
	taskKey := "task-abc"

	UpdateSession(sessionID, taskKey, fakeFacts(EvidenceStackTrace), cfg)

	level, info, tracked := CheckSessionEscalation(sessionID, taskKey, fakeFacts(EvidenceTestFailure), cfg)
	if !tracked {
		t.Fatal("expected tracked session")
	}
	if level != "hard" {
		t.Errorf("expected hard escalation, got %q", level)
	}
	if !info.UsedPreviousState {
		t.Error("expected UsedPreviousState=true")
	}
	if !info.UsedCurrentFrame {
		t.Error("expected UsedCurrentFrame=true")
	}
	if info.ProspectiveFailureCount != 2 {
		t.Errorf("expected ProspectiveFailureCount=2, got %d", info.ProspectiveFailureCount)
	}
	if !info.Escalated {
		t.Error("expected Escalated=true")
	}
}

func TestProspectiveEscalationThirdFailure(t *testing.T) {
	cfg := setupSessionTest()
	sessionID := "sess-2"
	taskKey := "task-xyz"

	UpdateSession(sessionID, taskKey, fakeFacts(EvidenceStackTrace), cfg)
	UpdateSession(sessionID, taskKey, fakeFacts(EvidenceCompileError), cfg)

	level, info, tracked := CheckSessionEscalation(sessionID, taskKey, fakeFacts(EvidenceTestFailure), cfg)
	if !tracked {
		t.Fatal("expected tracked session")
	}
	if level != "critical" {
		t.Errorf("expected critical escalation, got %q", level)
	}
	if info.ProspectiveFailureCount != 3 {
		t.Errorf("expected ProspectiveFailureCount=3, got %d", info.ProspectiveFailureCount)
	}
}

func TestNoEscalationBelowThreshold(t *testing.T) {
	cfg := setupSessionTest()
	sessionID := "sess-3"
	taskKey := "task-low"

	level, info, tracked := CheckSessionEscalation(sessionID, taskKey, fakeFacts(EvidenceToolError), cfg)
	if !tracked {
		t.Fatal("expected tracked session")
	}
	if level != "" {
		t.Errorf("expected no escalation (previous=0, current=1 < hard_after=2), got %q", level)
	}
	if info.ProspectiveFailureCount != 1 {
		t.Errorf("expected ProspectiveFailureCount=1, got %d", info.ProspectiveFailureCount)
	}
}

func TestNewTaskKeyResetsCounts(t *testing.T) {
	cfg := setupSessionTest()
	sessionID := "sess-4"

	UpdateSession(sessionID, "task-a", fakeFacts(EvidenceStackTrace), cfg)
	UpdateSession(sessionID, "task-a", fakeFacts(EvidenceCompileError), cfg)
	UpdateSession(sessionID, "task-a", fakeFacts(EvidenceTestFailure), cfg)

	level, info, _ := CheckSessionEscalation(sessionID, "task-b", fakeFacts(EvidenceToolError), cfg)
	if info.FailureCount != 0 {
		t.Errorf("new task should reset counters, got FailureCount=%d", info.FailureCount)
	}
	if level != "" {
		t.Errorf("new task should not escalate, got %q", level)
	}
}

func TestNoSessionHeaderStillTracksFrameEvidence(t *testing.T) {
	cfg := setupSessionTest()

	_, info, tracked := CheckSessionEscalation("", "task-x", fakeFacts(EvidenceStackTrace), cfg)
	if tracked {
		t.Error("no session ID should not track")
	}
	if info.SessionID != "" {
		t.Errorf("SessionID should be empty, got %q", info.SessionID)
	}
}

func TestSameTaskPreservesCountsAcrossFrames(t *testing.T) {
	cfg := setupSessionTest()
	sessionID := "sess-5"
	taskKey := "task-same"

	UpdateSession(sessionID, taskKey, fakeFacts(EvidenceStackTrace), cfg)

	_, info, _ := CheckSessionEscalation(sessionID, taskKey, fakeFacts(EvidenceToolError), cfg)
	if info.FailureCount != 1 {
		t.Errorf("expected FailureCount=1, got %d", info.FailureCount)
	}
	if !info.UsedPreviousState {
		t.Error("expected UsedPreviousState=true")
	}
}

func TestMultipleEvidenceTypesInOneFrame(t *testing.T) {
	cfg := setupSessionTest()
	sessionID := "sess-6"
	taskKey := "task-multi"

	facts := fakeFacts(EvidenceStackTrace, EvidenceToolError, EvidenceTestFailure)

	level, info, _ := CheckSessionEscalation(sessionID, taskKey, facts, cfg)
	if info.ProspectiveFailureCount != 3 {
		t.Errorf("expected ProspectiveFailureCount=3, got %d", info.ProspectiveFailureCount)
	}
	if level != "critical" {
		t.Errorf("expected critical escalation (3 failures in one frame), got %q", level)
	}
}

func TestDecayOnSuccess(t *testing.T) {
	cfg := setupSessionTest()
	sessionID := "sess-7"
	taskKey := "task-decay"

	UpdateSession(sessionID, taskKey, fakeFacts(EvidenceStackTrace), cfg)
	UpdateSession(sessionID, taskKey, fakeFacts(EvidenceCompileError), cfg)

	UpdateSession(sessionID, taskKey, Facts{}, cfg)

	level, info, _ := CheckSessionEscalation(sessionID, taskKey, fakeFacts(EvidenceTestFailure), cfg)
	if info.FailureCount != 1 {
		t.Errorf("expected FailureCount=1 after decay, got %d", info.FailureCount)
	}
	if level != "hard" {
		t.Errorf("expected hard (prev=1 + current=1 = 2 >= hard_after), got %q", level)
	}
}
