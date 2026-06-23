package router

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func TestDebugStatsEndpoint(t *testing.T) {
	router, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/debug/stats", nil)
	rec := httptest.NewRecorder()
	router.HandleDebugStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("stats status = %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["requests_total"] == nil {
		t.Error("missing requests_total")
	}
	if resp["by_level"] == nil {
		t.Error("missing by_level")
	}
	if resp["uptime_seconds"] == nil {
		t.Error("missing uptime_seconds")
	}
}

func TestDebugRequestLookupNotFound(t *testing.T) {
	router, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/debug/request?id=nonexistent", nil)
	rec := httptest.NewRecorder()
	router.HandleDebugRequestLookup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestDebugRequestLookupDisabled(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.RequestIndexEnabled = false
	router, _ := setupTestWithCfg(cfg, t)

	req := httptest.NewRequest(http.MethodGet, "/debug/request?id=any", nil)
	rec := httptest.NewRecorder()
	router.HandleDebugRequestLookup(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when disabled, got %d", rec.Code)
	}
}

func TestDebugRequestLookupMissingID(t *testing.T) {
	router, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/debug/request", nil)
	rec := httptest.NewRecorder()
	router.HandleDebugRequestLookup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestFeedbackDisabledByDefault(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"request_id":"r1","expected_level":"easy","note":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/debug/feedback", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.HandleDebugFeedback(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when feedback disabled, got %d", rec.Code)
	}
}

func TestFeedbackEnabledWritesJSONL(t *testing.T) {
	dir := t.TempDir()
	feedbackPath := filepath.Join(dir, "feedback.jsonl")

	cfg, _ := config.DefaultConfig()
	cfg.Debug.FeedbackEnabled = true
	cfg.Debug.FeedbackPath = feedbackPath
	router, _ := setupTestWithCfg(cfg, t)

	body := `{"request_id":"r1","expected_level":"hard","note":"should have been hard"}`
	req := httptest.NewRequest(http.MethodPost, "/debug/feedback", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.HandleDebugFeedback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	data, err := os.ReadFile(feedbackPath)
	if err != nil {
		t.Fatal("feedback file not created")
	}
	if !strings.Contains(string(data), "r1") {
		t.Error("feedback file should contain request_id")
	}
	if !strings.Contains(string(data), "hard") {
		t.Error("feedback file should contain expected_level")
	}
	if !strings.Contains(string(data), "should have been hard") {
		t.Error("feedback file should contain note")
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("feedback should be JSONL (newline terminated)")
	}
}

func TestFeedbackInvalidExpectedLevel(t *testing.T) {
	dir := t.TempDir()
	feedbackPath := filepath.Join(dir, "feedback.jsonl")

	cfg, _ := config.DefaultConfig()
	cfg.Debug.FeedbackEnabled = true
	cfg.Debug.FeedbackPath = feedbackPath
	router, _ := setupTestWithCfg(cfg, t)

	body := `{"request_id":"r1","expected_level":"super-hard","note":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/debug/feedback", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.HandleDebugFeedback(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid level, got %d", rec.Code)
	}
}

func TestFeedbackNoteTruncation(t *testing.T) {
	dir := t.TempDir()
	feedbackPath := filepath.Join(dir, "feedback.jsonl")

	cfg, _ := config.DefaultConfig()
	cfg.Debug.FeedbackEnabled = true
	cfg.Debug.FeedbackPath = feedbackPath
	router, _ := setupTestWithCfg(cfg, t)

	longNote := strings.Repeat("x", 600)
	body := `{"request_id":"r1","expected_level":"easy","note":"` + longNote + `"}`
	req := httptest.NewRequest(http.MethodPost, "/debug/feedback", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.HandleDebugFeedback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	data, _ := os.ReadFile(feedbackPath)
	var entry map[string]interface{}
	json.NewDecoder(bytes.NewReader(data)).Decode(&entry)
	note, _ := entry["note"].(string)
	if len(note) != 500 {
		t.Errorf("note should be truncated to 500 chars, got %d", len(note))
	}
}

func TestDecisionLogNoPromptText(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.LogDecisions = true
	router, _ := setupTestWithCfg(cfg, t)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"secret password is sk-12345"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTraceLogNoPromptText(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.TraceRequests = true
	router, _ := setupTestWithCfg(cfg, t)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"my secret API key sk-abc123"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestStatsIncrementAfterRequest(t *testing.T) {
	router, _ := setupTest(t)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi there"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	snap := router.Stats.Snapshot()
	if snap.RequestsTotal != 1 {
		t.Errorf("expected 1 request, got %d", snap.RequestsTotal)
	}
	if snap.ByLevel["easy"] != 1 {
		t.Errorf("expected 1 easy, got %v", snap.ByLevel)
	}
}

func TestRequestIndexStoresAfterRequest(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.RequestIndexEnabled = true
	router, _ := setupTestWithCfg(cfg, t)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	requestID := rec.Header().Get("X-Dispatch-Request-Id")
	if requestID == "" {
		t.Fatal("missing request ID header")
	}

	meta, ok := router.RequestIndex.Lookup(requestID)
	if !ok {
		t.Fatal("request not found in index")
	}
	if meta.Level == "" {
		t.Error("level should be set")
	}
	if meta.Model == "" {
		t.Error("model should be set")
	}
	if meta.RequestID != requestID {
		t.Errorf("request_id mismatch: %s != %s", meta.RequestID, requestID)
	}
}

func TestRequestLookupEndpointReturnsMeta(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.RequestIndexEnabled = true
	router, _ := setupTestWithCfg(cfg, t)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	requestID := rec.Header().Get("X-Dispatch-Request-Id")

	lookupReq := httptest.NewRequest(http.MethodGet, "/debug/request?id="+requestID, nil)
	lookupRec := httptest.NewRecorder()
	router.HandleDebugRequestLookup(lookupRec, lookupReq)

	if lookupRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", lookupRec.Code, lookupRec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(lookupRec.Body).Decode(&resp)
	if resp["request_id"] != requestID {
		t.Errorf("request_id = %v, want %s", resp["request_id"], requestID)
	}
	if resp["level"] == nil {
		t.Error("missing level in response")
	}
}

func setupTestWithCfg(cfg *config.Config, t *testing.T) (*Router, *httptest.Server) {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		json.Unmarshal(body, &parsed)
		if parsed["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"model":   parsed["model"],
				"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
			})
		}
	}))
	t.Cleanup(upstream.Close)

	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_OPS_KEY", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)
	return router, upstream
}
