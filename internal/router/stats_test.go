package router

import (
	"testing"
	"time"
)

func TestStatsRecordAndSnapshot(t *testing.T) {
	s := NewStats()

	s.Record("easy", "model-a", 200, false, 1_000_000, false, false, false, false, false, "", "", "", false)
	s.Record("hard", "model-b", 200, true, 2_000_000, true, true, true, true, true, "", "", "", false)

	snap := s.Snapshot()

	if snap.RequestsTotal != 2 {
		t.Errorf("expected 2 requests, got %d", snap.RequestsTotal)
	}
	if snap.ByLevel["easy"] != 1 || snap.ByLevel["hard"] != 1 {
		t.Errorf("by_level wrong: %v", snap.ByLevel)
	}
	if snap.ByModel["model-a"] != 1 || snap.ByModel["model-b"] != 1 {
		t.Errorf("by_model wrong: %v", snap.ByModel)
	}
	if snap.ByStatus[200] != 2 {
		t.Errorf("by_status wrong: %v", snap.ByStatus)
	}
	if snap.StreamCount != 1 {
		t.Errorf("stream_count = %d, want 1", snap.StreamCount)
	}
	if snap.NonstreamCount != 1 {
		t.Errorf("nonstream_count = %d, want 1", snap.NonstreamCount)
	}
	if snap.SessionEscalations != 1 {
		t.Errorf("session_escalations = %d, want 1", snap.SessionEscalations)
	}
	if snap.CriticalGates != 1 {
		t.Errorf("critical_gates = %d, want 1", snap.CriticalGates)
	}
	if snap.ContinuationDetected != 1 {
		t.Errorf("continuation_detected = %d, want 1", snap.ContinuationDetected)
	}
	if snap.LengthCappedCount != 1 {
		t.Errorf("length_capped = %d, want 1", snap.LengthCappedCount)
	}
	if snap.TopicIgnoredCount != 1 {
		t.Errorf("topic_ignored = %d, want 1", snap.TopicIgnoredCount)
	}
	if snap.AvgRouteDurationMs <= 0 {
		t.Errorf("avg_route_duration_ms should be > 0, got %f", snap.AvgRouteDurationMs)
	}
	if snap.UptimeSeconds < 0 {
		t.Errorf("uptime_seconds should be >= 0, got %d", snap.UptimeSeconds)
	}
}

func TestStatsRecordReload(t *testing.T) {
	s := NewStats()
	now := time.Now().Unix()
	s.RecordReload(now)

	snap := s.Snapshot()
	if snap.ConfigReloadCount != 1 {
		t.Errorf("config_reload_count = %d, want 1", snap.ConfigReloadCount)
	}
	if snap.LastConfigReloadUnix != now {
		t.Errorf("last_config_reload_unix = %d, want %d", snap.LastConfigReloadUnix, now)
	}
}

func TestStatsConcurrency(t *testing.T) {
	s := NewStats()
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				s.Record("easy", "model-a", 200, false, 1000, false, false, false, false, false, "", "", "", false)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
	snap := s.Snapshot()
	if snap.RequestsTotal != 1000 {
		t.Errorf("expected 1000, got %d", snap.RequestsTotal)
	}
}
