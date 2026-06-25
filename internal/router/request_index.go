package router

import (
	"sync"
	"time"
)

type RequestMeta struct {
	RequestID         string    `json:"request_id"`
	Timestamp         time.Time `json:"timestamp"`
	Level             string    `json:"level"`
	Model             string    `json:"model"`
	Status            int       `json:"status"`
	LatestUserIndex   int       `json:"latest_user_index"`
	TaskBoundaryIndex int       `json:"task_boundary_index"`
	TaskKey           string    `json:"task_key"`
	LatestUserHash    string    `json:"latest_user_hash"`
	FrameHash         string    `json:"frame_hash"`
	ReasonSummary     []string  `json:"reason_summary"`
	CriticalGate      string    `json:"critical_gate,omitempty"`
	SessionEscalated  bool      `json:"session_escalated"`

	UpstreamErrorCode    int    `json:"upstream_error_code,omitempty"`
	UpstreamErrorType    string `json:"upstream_error_type,omitempty"`
	UpstreamProviderCode string `json:"upstream_provider_code,omitempty"`
	UpstreamProvider     string `json:"upstream_provider,omitempty"`
	UpstreamIsBYOK       string `json:"upstream_is_byok,omitempty"`
	UpstreamRetryAfter   string `json:"upstream_retry_after,omitempty"`
	UpstreamRetryable    string `json:"upstream_retryable,omitempty"`
	UpstreamRawTruncated string `json:"upstream_raw_truncated,omitempty"`
	UpstreamErrorMsg     string `json:"upstream_error_message_truncated,omitempty"`
	EmbeddedError        bool   `json:"embedded_error"`
}

type requestIndex struct {
	mu      sync.RWMutex
	entries map[string]*RequestMeta
	order   []string
	maxSize int
}

func newRequestIndex(maxSize int) *requestIndex {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &requestIndex{
		entries: make(map[string]*RequestMeta, maxSize),
		maxSize: maxSize,
	}
}

func (ri *requestIndex) Store(meta *RequestMeta) {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	if len(ri.order) >= ri.maxSize {
		oldest := ri.order[0]
		delete(ri.entries, oldest)
		ri.order = ri.order[1:]
	}
	ri.entries[meta.RequestID] = meta
	ri.order = append(ri.order, meta.RequestID)
}

func (ri *requestIndex) Lookup(requestID string) (*RequestMeta, bool) {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	meta, ok := ri.entries[requestID]
	return meta, ok
}

func (ri *requestIndex) Len() int {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return len(ri.entries)
}
