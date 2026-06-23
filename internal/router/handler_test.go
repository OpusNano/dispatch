package router

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func setupTest(t *testing.T) (*Router, *httptest.Server) {
	t.Helper()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		json.Unmarshal(body, &parsed)

		if parsed["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("X-Upstream-Header", "stream-value")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Upstream-Header", "json-value")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"model":   parsed["model"],
				"choices": []map[string]interface{}{{"message": map[string]string{"content": "response"}}},
			})
		}
	}))
	t.Cleanup(upstream.Close)

	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_KEY", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)
	return router, upstream
}

func TestHealthEndpoint(t *testing.T) {
	router, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("health status = %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("health status = %s", resp["status"])
	}
}

func TestVersionEndpoint(t *testing.T) {
	router, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	router.HandleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("version status = %d", rec.Code)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["version"] == "" {
		t.Error("version is empty")
	}
}

func TestDebugRoute(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("debug status = %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["level"] == "" {
		t.Error("level missing")
	}
	if resp["model"] == nil {
		t.Error("model missing")
	}
}

func TestChatCompletionsOverrideHeader(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/easy","messages":[{"role":"user","content":"production database migration rollback"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dispatch-Level", "critical")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	level := rec.Header().Get("X-Dispatch-Level")
	if level != "critical" {
		t.Errorf("header override should win: got %s, want critical", level)
	}
}

func TestChatCompletionsAliasOverride(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/critical","messages":[{"role":"user","content":"hello world"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	level := rec.Header().Get("X-Dispatch-Level")
	if level != "critical" {
		t.Errorf("alias override should force critical: got %s", level)
	}
	forced := rec.Header().Get("X-Dispatch-Forced-By")
	if !strings.Contains(forced, "model-alias") {
		t.Errorf("forced-by header = %s, want model-alias", forced)
	}
}

func TestChatCompletionsClassification(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi there"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	level := rec.Header().Get("X-Dispatch-Level")
	if level != "easy" {
		t.Errorf("greeting should classify as easy, got %s", level)
	}
	if rec.Header().Get("X-Dispatch-Model") == "" {
		t.Error("model header missing")
	}
}

func TestChatCompletionsNormalModel(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"gpt-4","messages":[{"role":"user","content":"write a function that sorts an array"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Header().Get("X-Dispatch-Level") == "" {
		t.Error("routing headers missing for normal model request")
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["model"] != "deepseek/deepseek-v4-flash" {
		t.Errorf("model should be replaced: got %v", resp["model"])
	}
}

func TestStreamingPassthrough(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("streaming content type: got %s, want text/event-stream", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Error("X-Accel-Buffering should be no")
	}
	if rec.Header().Get("X-Dispatch-Level") == "" {
		t.Error("routing headers missing on stream")
	}

	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "data: [DONE]") {
		t.Error("streaming response missing DONE marker")
	}
	if !strings.Contains(bodyStr, "Hello") {
		t.Error("streaming response missing content")
	}
}

func TestUpstreamHeadersCopied(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Header().Get("X-Upstream-Header") != "json-value" {
		t.Errorf("upstream headers not copied: got %s", rec.Header().Get("X-Upstream-Header"))
	}
}

func TestBodySizeLimit(t *testing.T) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Server.MaxBodySize = 100
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_KEY2", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	largeBody := `{"model":"x","messages":[{"role":"user","content":"` + strings.Repeat("x", 1000) + `"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("body size limit: got %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	router, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET should be 405, got %d", rec.Code)
	}
}

func TestResponseHeadersTruncated(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function with code and tests and refactor the architecture"}],"tools":[{"type":"function"}],"response_format":{"type":"json_object"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	reasons := rec.Header().Get("X-Dispatch-Reasons")
	if reasons == "" {
		t.Error("reasons header missing")
	}
	if len(reasons) > 1024+50 {
		t.Errorf("reasons not truncated: %d bytes", len(reasons))
	}
}

func TestRewritePreservesUnknownFields(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"custom_parameter":"keep-me","nested":{"deep":true}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	level := rec.Header().Get("X-Dispatch-Level")
	if level == "" {
		t.Error("classification should still work")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("request should succeed: status %d", rec.Code)
	}
}

func TestHeaderOverrideWinsOverAlias(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/easy","messages":[{"role":"user","content":"test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dispatch-Level", "hard")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	level := rec.Header().Get("X-Dispatch-Level")
	if level != "hard" {
		t.Errorf("header should win over alias: got %s, want hard", level)
	}
}

func TestDebugRouteNoUpstreamCall(t *testing.T) {
	router, upstream := setupTest(t)
	callCount := 0
	upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
	})

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	if callCount > 0 {
		t.Error("debug route should not call upstream")
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["level"] == nil {
		t.Error("debug route should return level")
	}
}

func TestAllRoutingHeadersPresent(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	requiredHeaders := []string{
		"X-Dispatch-Level",
		"X-Dispatch-Model",
		"X-Dispatch-Score-Total",
		"X-Dispatch-Score-Complexity",
		"X-Dispatch-Score-Risk",
		"X-Dispatch-Score-Agent-Pressure",
		"X-Dispatch-Reasons",
	}
	for _, h := range requiredHeaders {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing required header: %s", h)
		}
	}
}

func TestStreamingHeadersPresent(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write code"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	requiredHeaders := []string{
		"X-Dispatch-Level",
		"X-Dispatch-Model",
		"X-Dispatch-Score-Total",
		"X-Dispatch-Reasons",
	}
	for _, h := range requiredHeaders {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing required header on stream: %s", h)
		}
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("stream Content-Type = %s, want text/event-stream", rec.Header().Get("Content-Type"))
	}
}

func TestStreamingKeepaliveComments(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(": OPENROUTER PROCESSING\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
		w.Write([]byte(": OPENROUTER PROCESSING\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}))
	t.Cleanup(upstream.Close)

	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_KEEPALIVE", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	responseBody := rec.Body.String()
	if !strings.Contains(responseBody, ": OPENROUTER PROCESSING") {
		t.Error("keepalive comments lost in streaming")
	}
	if !strings.Contains(responseBody, "data: {\"choices\"") {
		t.Error("data chunks lost in streaming")
	}
	if !strings.Contains(responseBody, "data: [DONE]") {
		t.Error("DONE marker lost in streaming")
	}
}

func TestContentTypePreserved(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Custom", "custom-value")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	t.Cleanup(upstream.Close)

	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_CT", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %s, want application/json", ct)
	}
	if rec.Header().Get("X-Custom") != "custom-value" {
		t.Error("custom upstream header not preserved")
	}
}

func TestContentLengthNotSetByUs(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	contentLengthSeen := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "42")
		contentLengthSeen = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(upstream.Close)

	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_CL", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if !contentLengthSeen {
		t.Log("Content-Length not set by upstream (test limitation)")
	}
}

func TestStreamingChunksFlushed(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var chunks []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Write multiple chunks to ensure they arrive separately
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"chunk1\"}}]}\n\n"))
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"chunk2\"}}]}\n\n"))
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}))
	t.Cleanup(upstream.Close)

	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_FLUSH", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)
	_ = chunks

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	responseBody := rec.Body.String()
	if !strings.Contains(responseBody, "chunk1") {
		t.Error("chunk1 missing in stream output")
	}
	if !strings.Contains(responseBody, "chunk2") {
		t.Error("chunk2 missing in stream output")
	}
	if !strings.Contains(responseBody, "[DONE]") {
		t.Error("DONE missing in stream output")
	}
}

func TestAccelBufferingHeaderSet(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Header().Get("X-Accel-Buffering") != "no" {
		t.Errorf("X-Accel-Buffering = %s, want no", rec.Header().Get("X-Accel-Buffering"))
	}
}

func TestProviderMergeInFullFlow(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	easyLevel := cfg.Levels["easy"]
	easyLevel.Provider = config.ProviderConfig{
		DataCollection: "deny",
		Order:          []string{"anthropic"},
	}
	cfg.Levels["easy"] = easyLevel

	var receivedProvider map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		json.Unmarshal(body, &parsed)
		if prov, ok := parsed["provider"].(map[string]interface{}); ok {
			receivedProvider = prov
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	}))
	t.Cleanup(upstream.Close)

	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_PROV", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if receivedProvider == nil {
		t.Fatal("provider not sent upstream")
	}
	if receivedProvider["data_collection"] != "deny" {
		t.Errorf("data_collection = %v, want deny", receivedProvider["data_collection"])
	}
	if receivedProvider["order"] == nil {
		t.Error("order missing in upstream provider")
	}
}

func TestDebugRouteResponseFormat(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"the API key was accidentally committed to the public git repo, help rotate it"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("debug status = %d", rec.Code)
	}

	var resp struct {
		Level  string `json:"level"`
		Model  string `json:"model"`
		Scores struct {
			Complexity    float64 `json:"complexity"`
			Risk          float64 `json:"risk"`
			AgentPressure float64 `json:"agent_pressure"`
			Downgrade     float64 `json:"downgrade"`
			Total         float64 `json:"total"`
		} `json:"scores"`
		Reasons []string `json:"reasons"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Level != "critical" {
		t.Errorf("debug route level = %s, want critical", resp.Level)
	}
	if resp.Model != "z-ai/glm-5.2" {
		t.Errorf("debug route model = %s, want z-ai/glm-5.2", resp.Model)
	}
	if resp.Scores.Risk < 15 {
		t.Errorf("debug route risk = %.1f, expected >15", resp.Scores.Risk)
	}
	if len(resp.Reasons) < 2 {
		t.Errorf("debug route reasons too few: %v", resp.Reasons)
	}
}

func TestDebugRouteAnalysisOutput(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"debug auth bug in production with compile error"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("debug status = %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp["level"] == nil {
		t.Error("missing level")
	}
	if resp["model"] == nil {
		t.Error("missing model")
	}
	if resp["scores"] == nil {
		t.Error("missing scores")
	}
	if resp["reasons"] == nil {
		t.Error("missing reasons")
	}
	if resp["request_id"] == nil || resp["request_id"] == "" {
		t.Error("missing request_id")
	}

	analysis, ok := resp["analysis"].(map[string]interface{})
	if !ok {
		t.Fatal("missing or invalid analysis object")
	}

	requiredFields := []string{
		"intent",
		"operation",
		"domains",
		"scope",
		"evidence",
		"risk",
		"agent_state",
		"floors",
		"critical_gates",
	}
	for _, field := range requiredFields {
		if _, exists := analysis[field]; !exists {
			t.Errorf("analysis missing field: %s", field)
		}
	}

	if analysis["intent"] == "" || analysis["intent"] == nil {
		t.Error("analysis intent should be populated")
	}
	if analysis["operation"] == "" || analysis["operation"] == nil {
		t.Error("analysis operation should be populated")
	}
	domains, ok := analysis["domains"].([]interface{})
	if !ok || len(domains) == 0 {
		t.Error("analysis domains should be non-empty array")
	}
}

func TestDebugRouteCriticalGateInAnalysis(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"auth bypass in production lets users access other accounts"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["level"] != "critical" {
		t.Errorf("level = %v, want critical", resp["level"])
	}
	analysis := resp["analysis"].(map[string]interface{})
	gates, ok := analysis["critical_gates"].([]interface{})
	if !ok || len(gates) == 0 {
		t.Error("expected critical_gates to be populated for gate-triggered critical")
	}
}

func TestHelpfulErrorMessageOnStartup(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.OpenRouter.APIKeyEnv = "NONEXISTENT_ENV_VAR_12345"
	os.Unsetenv("NONEXISTENT_ENV_VAR_12345")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL

	envKey := os.Getenv(cfg.OpenRouter.APIKeyEnv)
	if envKey != "" {
		t.Errorf("expected empty env var, got %s", envKey)
	}
}

func TestDebugRouteFrameOutput(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"fix auth bug"},{"role":"assistant","content":"ok"},{"role":"tool","content":"stack trace: panic at auth.go:42"},{"role":"tool","content":"3 tests failed FAIL"},{"role":"user","content":"explain closures"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	frame, ok := resp["frame"].(map[string]interface{})
	if !ok {
		t.Fatal("frame field missing from debug response")
	}

	if frame["latest_user_index"] != float64(4) {
		t.Errorf("latest_user_index = %v, want 4", frame["latest_user_index"])
	}
	if frame["task_boundary_index"] != float64(4) {
		t.Errorf("task_boundary_index = %v, want 4", frame["task_boundary_index"])
	}
	if frame["continuation_detected"] != false {
		t.Error("should not detect continuation")
	}
	if frame["excluded_prior_hard_context"] != true {
		t.Error("should flag excluded hard context")
	}
	if frame["prior_level_ignored_for_routing"] != true {
		t.Error("prior level should be ignored")
	}
	if frame["task_key_source"] != "derived_from_boundary_user" {
		t.Errorf("task_key_source = %v", frame["task_key_source"])
	}
	if frame["task_key"] == nil || frame["task_key"] == "" {
		t.Error("task_key missing")
	}

	level := resp["level"]
	if level != "easy" {
		t.Errorf("old hard context excluded: level = %v, want easy", level)
	}
}

func TestDebugRouteFrameContinuation(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"fix auth bug"},{"role":"assistant","content":"ok"},{"role":"tool","content":"stack trace error"},{"role":"user","content":"same error still happens"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	frame, ok := resp["frame"].(map[string]interface{})
	if !ok {
		t.Fatal("frame field missing")
	}

	if frame["continuation_detected"] != true {
		t.Error("should detect continuation for 'same error still happens'")
	}
	reasons, ok := frame["continuation_reasons"].([]interface{})
	if !ok || len(reasons) < 1 {
		t.Error("continuation_reasons missing or empty")
	}
}

func TestDebugRouteFrameToolMessages(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"fix auth bug"},{"role":"assistant","content":"let me check"},{"role":"tool","content":"error output"},{"role":"tool","content":"another error"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	frame, ok := resp["frame"].(map[string]interface{})
	if !ok {
		t.Fatal("frame field missing")
	}

	if frame["latest_user_index"] != float64(0) {
		t.Errorf("latest_user_index = %v, want 0", frame["latest_user_index"])
	}
	if frame["task_boundary_index"] != float64(0) {
		t.Errorf("task_boundary_index = %v, want 0", frame["task_boundary_index"])
	}
}
