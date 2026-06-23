package router

import (
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	mu sync.RWMutex

	startTime          time.Time
	requestsTotal      atomic.Int64
	streamCount        atomic.Int64
	nonstreamCount     atomic.Int64
	sessionEscalations atomic.Int64
	criticalGates      atomic.Int64
	continuationCount  atomic.Int64
	lengthCappedCount  atomic.Int64
	topicIgnoredCount  atomic.Int64
	configReloadCount  atomic.Int64
	lastConfigReload   atomic.Int64

	byLevel              map[string]*atomic.Int64
	byModel              map[string]*atomic.Int64
	byStatus             map[int]*atomic.Int64
	totalRouteDurationNs atomic.Int64
	routeDurationCount   atomic.Int64
}

func NewStats() *Stats {
	return &Stats{
		startTime: time.Now(),
		byLevel:   make(map[string]*atomic.Int64),
		byModel:   make(map[string]*atomic.Int64),
		byStatus:  make(map[int]*atomic.Int64),
	}
}

func (s *Stats) Record(level, model string, statusCode int, isStream bool, routeDurationNs int64, sessionEscalated, gateFired, continuationDetected, lengthCapped, topicIgnored bool) {
	s.requestsTotal.Add(1)
	if isStream {
		s.streamCount.Add(1)
	} else {
		s.nonstreamCount.Add(1)
	}
	if sessionEscalated {
		s.sessionEscalations.Add(1)
	}
	if gateFired {
		s.criticalGates.Add(1)
	}
	if continuationDetected {
		s.continuationCount.Add(1)
	}
	if lengthCapped {
		s.lengthCappedCount.Add(1)
	}
	if topicIgnored {
		s.topicIgnoredCount.Add(1)
	}

	s.mu.Lock()
	if _, ok := s.byLevel[level]; !ok {
		s.byLevel[level] = &atomic.Int64{}
	}
	if _, ok := s.byModel[model]; !ok {
		s.byModel[model] = &atomic.Int64{}
	}
	if _, ok := s.byStatus[statusCode]; !ok {
		s.byStatus[statusCode] = &atomic.Int64{}
	}
	s.mu.Unlock()

	s.byLevel[level].Add(1)
	s.byModel[model].Add(1)
	s.byStatus[statusCode].Add(1)
	s.totalRouteDurationNs.Add(routeDurationNs)
	s.routeDurationCount.Add(1)
}

func (s *Stats) RecordReload(unixTime int64) {
	s.configReloadCount.Add(1)
	s.lastConfigReload.Store(unixTime)
}

type StatsSnapshot struct {
	RequestsTotal        int64            `json:"requests_total"`
	ByLevel              map[string]int64 `json:"by_level"`
	ByModel              map[string]int64 `json:"by_model"`
	ByStatus             map[int]int64    `json:"by_status"`
	StreamCount          int64            `json:"stream_count"`
	NonstreamCount       int64            `json:"nonstream_count"`
	SessionEscalations   int64            `json:"session_escalations"`
	CriticalGates        int64            `json:"critical_gates"`
	ContinuationDetected int64            `json:"continuation_detected_count"`
	LengthCappedCount    int64            `json:"length_capped_count"`
	TopicIgnoredCount    int64            `json:"topic_ignored_count"`
	AvgRouteDurationMs   float64          `json:"avg_route_duration_ms"`
	UptimeSeconds        int64            `json:"uptime_seconds"`
	ConfigReloadCount    int64            `json:"config_reload_count"`
	LastConfigReloadUnix int64            `json:"last_config_reload_unix"`
}

func (s *Stats) Snapshot() StatsSnapshot {
	s.mu.RLock()
	byLevel := make(map[string]int64, len(s.byLevel))
	for k, v := range s.byLevel {
		byLevel[k] = v.Load()
	}
	byModel := make(map[string]int64, len(s.byModel))
	for k, v := range s.byModel {
		byModel[k] = v.Load()
	}
	byStatus := make(map[int]int64, len(s.byStatus))
	for k, v := range s.byStatus {
		byStatus[k] = v.Load()
	}
	s.mu.RUnlock()

	var avgMs float64
	count := s.routeDurationCount.Load()
	if count > 0 {
		totalNs := s.totalRouteDurationNs.Load()
		avgMs = float64(totalNs) / float64(count) / 1e6
	}

	return StatsSnapshot{
		RequestsTotal:        s.requestsTotal.Load(),
		ByLevel:              byLevel,
		ByModel:              byModel,
		ByStatus:             byStatus,
		StreamCount:          s.streamCount.Load(),
		NonstreamCount:       s.nonstreamCount.Load(),
		SessionEscalations:   s.sessionEscalations.Load(),
		CriticalGates:        s.criticalGates.Load(),
		ContinuationDetected: s.continuationCount.Load(),
		LengthCappedCount:    s.lengthCappedCount.Load(),
		TopicIgnoredCount:    s.topicIgnoredCount.Load(),
		AvgRouteDurationMs:   avgMs,
		UptimeSeconds:        int64(time.Since(s.startTime).Seconds()),
		ConfigReloadCount:    s.configReloadCount.Load(),
		LastConfigReloadUnix: s.lastConfigReload.Load(),
	}
}
