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

	apiKeyReloadCount atomic.Int64
	lastAPIKeyReload  atomic.Int64

	byLevel              map[string]*atomic.Int64
	byModel              map[string]*atomic.Int64
	byStatus             map[int]*atomic.Int64
	totalRouteDurationNs atomic.Int64
	routeDurationCount   atomic.Int64

	upstreamErrorsTotal     atomic.Int64
	upstreamRateLimitsTotal atomic.Int64
	upstream429Total        atomic.Int64
	upstream502Total        atomic.Int64
	upstream503Total        atomic.Int64
	upstreamEmbeddedTotal   atomic.Int64

	byUpstreamErrorType map[string]*atomic.Int64
	byUpstreamProvider  map[string]*atomic.Int64
	byUpstreamProvCode  map[string]*atomic.Int64

	localProxyErrorsTotal atomic.Int64

	apiKeyPresent     atomic.Bool
	apiKeyPrefixValid atomic.Bool
	apiKeyLength      atomic.Int64
}

func NewStats() *Stats {
	return &Stats{
		startTime:           time.Now(),
		byLevel:             make(map[string]*atomic.Int64),
		byModel:             make(map[string]*atomic.Int64),
		byStatus:            make(map[int]*atomic.Int64),
		byUpstreamErrorType: make(map[string]*atomic.Int64),
		byUpstreamProvider:  make(map[string]*atomic.Int64),
		byUpstreamProvCode:  make(map[string]*atomic.Int64),
	}
}

func (s *Stats) Record(level, model string, statusCode int, isStream bool, routeDurationNs int64, sessionEscalated, gateFired, continuationDetected, lengthCapped, topicIgnored bool, errorType string, providerName string, providerCode string, embeddedError bool) {
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

	if statusCode >= 400 || embeddedError {
		s.upstreamErrorsTotal.Add(1)
		switch {
		case statusCode == 429:
			s.upstream429Total.Add(1)
			s.upstreamRateLimitsTotal.Add(1)
		case statusCode == 502:
			s.upstream502Total.Add(1)
		case statusCode == 503:
			s.upstream503Total.Add(1)
		}
		if errorType == "rate_limit_exceeded" {
			s.upstreamRateLimitsTotal.Add(1)
		}
		if embeddedError {
			s.upstreamEmbeddedTotal.Add(1)
		}

		if errorType != "" {
			s.mu.Lock()
			if _, ok := s.byUpstreamErrorType[errorType]; !ok {
				s.byUpstreamErrorType[errorType] = &atomic.Int64{}
			}
			s.mu.Unlock()
			s.byUpstreamErrorType[errorType].Add(1)
		}
		if providerName != "" {
			s.mu.Lock()
			if _, ok := s.byUpstreamProvider[providerName]; !ok {
				s.byUpstreamProvider[providerName] = &atomic.Int64{}
			}
			s.mu.Unlock()
			s.byUpstreamProvider[providerName].Add(1)
		}
		if providerCode != "" {
			s.mu.Lock()
			if _, ok := s.byUpstreamProvCode[providerCode]; !ok {
				s.byUpstreamProvCode[providerCode] = &atomic.Int64{}
			}
			s.mu.Unlock()
			s.byUpstreamProvCode[providerCode].Add(1)
		}
	}
}

func (s *Stats) SetAPIKeyPresent(present bool) {
	s.apiKeyPresent.Store(present)
}

func (s *Stats) SetAPIKeyMeta(prefixValid bool, length int) {
	s.apiKeyPrefixValid.Store(prefixValid)
	s.apiKeyLength.Store(int64(length))
}

func (s *Stats) RecordLocalError() {
	s.localProxyErrorsTotal.Add(1)
}

func (s *Stats) RecordReload(unixTime int64) {
	s.configReloadCount.Add(1)
	s.lastConfigReload.Store(unixTime)
}

func (s *Stats) RecordAPIKeyReload(unixTime int64) {
	s.apiKeyReloadCount.Add(1)
	s.lastAPIKeyReload.Store(unixTime)
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

	UpstreamErrorsTotal    int64            `json:"upstream_errors_total"`
	ByUpstreamErrorType    map[string]int64 `json:"by_upstream_error_type"`
	ByUpstreamProvider     map[string]int64 `json:"by_upstream_provider"`
	ByUpstreamProviderCode map[string]int64 `json:"by_upstream_provider_code"`
	UpstreamRateLimits     int64            `json:"upstream_rate_limits_total"`
	Upstream429Total       int64            `json:"upstream_429_total"`
	Upstream502Total       int64            `json:"upstream_502_total"`
	Upstream503Total       int64            `json:"upstream_503_total"`
	UpstreamEmbeddedTotal  int64            `json:"upstream_embedded_errors_total"`
	LocalProxyErrorsTotal  int64            `json:"local_proxy_errors_total"`
	ApiKeyPresent          bool             `json:"api_key_present"`
	ApiKeyPrefixValid      bool             `json:"api_key_prefix_valid"`
	ApiKeyLength           int64            `json:"api_key_length"`
	ApiKeyReloadCount      int64            `json:"api_key_reload_count"`
	LastAPIKeyReloadUnix   int64            `json:"last_api_key_reload_unix"`
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
	byErrType := make(map[string]int64, len(s.byUpstreamErrorType))
	for k, v := range s.byUpstreamErrorType {
		byErrType[k] = v.Load()
	}
	byProv := make(map[string]int64, len(s.byUpstreamProvider))
	for k, v := range s.byUpstreamProvider {
		byProv[k] = v.Load()
	}
	byProvCode := make(map[string]int64, len(s.byUpstreamProvCode))
	for k, v := range s.byUpstreamProvCode {
		byProvCode[k] = v.Load()
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

		UpstreamErrorsTotal:    s.upstreamErrorsTotal.Load(),
		ByUpstreamErrorType:    byErrType,
		ByUpstreamProvider:     byProv,
		ByUpstreamProviderCode: byProvCode,
		UpstreamRateLimits:     s.upstreamRateLimitsTotal.Load(),
		Upstream429Total:       s.upstream429Total.Load(),
		Upstream502Total:       s.upstream502Total.Load(),
		Upstream503Total:       s.upstream503Total.Load(),
		UpstreamEmbeddedTotal:  s.upstreamEmbeddedTotal.Load(),
		LocalProxyErrorsTotal:  s.localProxyErrorsTotal.Load(),
		ApiKeyPresent:          s.apiKeyPresent.Load(),
		ApiKeyPrefixValid:      s.apiKeyPrefixValid.Load(),
		ApiKeyLength:           s.apiKeyLength.Load(),
		ApiKeyReloadCount:      s.apiKeyReloadCount.Load(),
		LastAPIKeyReloadUnix:   s.lastAPIKeyReload.Load(),
	}
}
