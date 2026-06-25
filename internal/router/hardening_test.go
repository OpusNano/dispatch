package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func TestInvalidJSONReturns400(t *testing.T) {
	router, _ := setupTest(t)
	body := `{invalid json not parseable}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON should return 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEmptyBodyReturns400(t *testing.T) {
	router, _ := setupTest(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty body should return 400, got %d", rec.Code)
	}
}

func TestMissingAPIKeyReturns503(t *testing.T) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called without API key")
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("missing API key should return 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	router, _ := setupTest(t)
	mux := http.NewServeMux()
	router.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent/path", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown path should return 404, got %d", rec.Code)
	}
}

func TestUpstreamErrorPassthrough(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{"401 unauthorized", http.StatusUnauthorized, `{"error":"invalid api key"}`},
		{"429 rate limited", http.StatusTooManyRequests, `{"error":"rate limited"}`},
		{"500 server error", http.StatusInternalServerError, `{"error":"internal"}`},
		{"502 bad gateway", http.StatusBadGateway, `{"error":"bad gateway"}`},
		{"503 service unavailable", http.StatusServiceUnavailable, `{"error":"unavailable"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := config.DefaultConfig()
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			t.Cleanup(upstream.Close)
			cfg.OpenRouter.BaseURL = upstream.URL
			client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
			router := New(cfg, client)

			body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.HandleChatCompletions(rec, req)

			if rec.Code != tt.statusCode {
				t.Errorf("upstream %d should pass through, got %d", tt.statusCode, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Errorf("upstream body should pass through, got %s", rec.Body.String())
			}
		})
	}
}

func TestUpstreamTimeoutReturnsCleanError(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL

	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	client.HTTPClient.Timeout = 50 * time.Millisecond
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("upstream timeout should return 502, got %d", rec.Code)
	}
}

func TestAuthorizationHeaderUsesServerKey(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var receivedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []map[string]interface{}{}})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "server-secret-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer client-provided-key")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if receivedAuth != "Bearer server-secret-key" {
		t.Errorf("upstream Authorization = %s, want Bearer server-secret-key", receivedAuth)
	}
}

func TestClientHeadersNotForwarded(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var receivedHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []map[string]interface{}{}})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer client-key")
	req.Header.Set("X-Custom-Client-Header", "should-not-forward")
	req.Header.Set("Cookie", "session=abc123")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if receivedHeaders.Get("X-Custom-Client-Header") != "" {
		t.Error("client X-Custom-Client-Header should not be forwarded upstream")
	}
	if receivedHeaders.Get("Cookie") != "" {
		t.Error("client Cookie should not be forwarded upstream")
	}
}

func TestContextCancellationPropagated(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	ctx, cancel := context.WithCancel(context.Background())
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	done := make(chan bool, 1)
	go func() {
		router.HandleChatCompletions(rec, req)
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler should return promptly after context cancellation")
	}
}

func TestMissingMessagesNoPanic(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked on missing messages: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("missing messages should still work, got %d", rec.Code)
	}
	level := rec.Header().Get("X-Dispatch-Level")
	if level == "" {
		t.Error("should classify even with missing messages")
	}
}

func TestContentArrayTextExtraction(t *testing.T) {
	router, upstream := setupTest(t)
	var receivedModel string
	upstream.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		json.Unmarshal(body, &parsed)
		receivedModel, _ = parsed["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []map[string]interface{}{}})
	})

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":[{"type":"text","text":"write a function to sort an array"}]}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("content array should work, got %d", rec.Code)
	}
	level := rec.Header().Get("X-Dispatch-Level")
	if level != "medium" && level != "easy" && level != "hard" {
		t.Errorf("content array text should be classified, got level=%s", level)
	}
	if receivedModel == "" {
		t.Error("model should be set in upstream request")
	}
}

func TestContentInputTextArrayExtraction(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":[{"type":"input_text","text":"DROP TABLE production data"}]}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	level := rec.Header().Get("X-Dispatch-Level")
	if level != "critical" {
		t.Errorf("input_text content should be classified as critical, got %s", level)
	}
}

func TestContentNullNoPanic(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":null}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked on null content: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("null content should work, got %d", rec.Code)
	}
}

func TestContentNumberNoPanic(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":42}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked on number content: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("number content should work, got %d", rec.Code)
	}
}

func TestContentBoolNoPanic(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":true}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked on bool content: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("bool content should work, got %d", rec.Code)
	}
}

func TestContentObjectNoPanic(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":{"text":"hello world"}}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked on object content: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("object content should work, got %d", rec.Code)
	}
}

func TestContentImageUrlNoPanic(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgo="}}]}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked on image_url content: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("image_url content should work, got %d", rec.Code)
	}
}

func TestAssistantToolCallMessageNoPanic(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"/etc/hosts\"}"}}]},{"role":"tool","tool_call_id":"call_1","content":"127.0.0.1 localhost"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panicked on tool call message: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("tool call message should work, got %d", rec.Code)
	}
}

func TestLargePayloadUnderLimit(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Server.MaxBodySize = 10 * 1024 * 1024

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []map[string]interface{}{}})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	largeContent := strings.Repeat("x", 1024*1024)
	body := fmt.Sprintf(`{"model":"dispatch/auto","messages":[{"role":"user","content":"%s"}],"stream":false}`, largeContent)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("large but under-limit payload should work, got %d", rec.Code)
	}
}

func TestRequestIDInResponse(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	reqID := rec.Header().Get("X-Dispatch-Request-Id")
	if reqID == "" {
		t.Error("X-Dispatch-Request-Id header missing")
	}
	if len(reqID) < 8 {
		t.Errorf("request ID too short: %s", reqID)
	}
}

func TestRequestIDUnique(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`

	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.HandleChatCompletions(rec, req)
		id := rec.Header().Get("X-Dispatch-Request-Id")
		if id == "" {
			t.Fatal("request ID empty")
		}
		if ids[id] {
			t.Errorf("request ID not unique: %s", id)
		}
		ids[id] = true
	}
}

func TestDebugRouteInvalidJSON(t *testing.T) {
	router, _ := setupTest(t)
	body := `{not valid json}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("invalid JSON on debug route should return 400, got %d", rec.Code)
	}
}

func TestDebugRouteRequestID(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	if rec.Header().Get("X-Dispatch-Request-Id") == "" {
		t.Error("debug route should have request ID header")
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["request_id"] == nil {
		t.Error("debug route response should include request_id field")
	}
}

func TestDebugRouteContentArray(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":[{"type":"text","text":"write a function to sort an array"}]}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("debug route with content array should work, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["level"] == nil {
		t.Error("should return classification")
	}
}

func TestHealthAllowsAnyMethod(t *testing.T) {
	router, _ := setupTest(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		req := httptest.NewRequest(method, "/health", nil)
		rec := httptest.NewRecorder()
		router.HandleHealth(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s /health should return 200, got %d", method, rec.Code)
		}
	}
}

func TestExtractTextFromContentDirect(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantText string
	}{
		{"plain string", `"hello world"`, "hello world"},
		{"text part array", `[{"type":"text","text":"hello from array"}]`, "hello from array"},
		{"input_text part array", `[{"type":"input_text","text":"hello input"}]`, "hello input"},
		{"image_url only (ignored)", `[{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]`, ""},
		{"mixed text and image", `[{"type":"text","text":"describe this"},{"type":"image_url","image_url":{"url":"..."}}]`, "describe this"},
		{"multiple text parts", `[{"type":"text","text":"part1"},{"type":"text","text":" part2"}]`, "part1 part2"},
		{"null content", `null`, ""},
		{"empty string", `""`, ""},
		{"number (ignored)", `42`, ""},
		{"boolean (ignored)", `true`, ""},
		{"object with text field", `{"text":"from object"}`, "from object"},
		{"empty array", `[]`, ""},
		{"mixed valid and invalid parts", `[{"type":"text","text":"keep"},42,{"type":"text","text":"also keep"}]`, "keepalso keep"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if err := json.Unmarshal([]byte(tt.json), &raw); err != nil {
				t.Fatalf("failed to parse test JSON: %v", err)
			}
			got := extractTextFromContent(raw)
			if got != tt.wantText {
				t.Errorf("extractTextFromContent(%s) = %q, want %q", tt.json, got, tt.wantText)
			}
		})
	}
}

func TestExtractTextFromToolCallsDirect(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		wantText string
	}{
		{"empty", "", ""},
		{"null", `null`, ""},
		{"single tool call", `[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"/etc/hosts\"}"}}]`, `read_file {"path":"/etc/hosts"}`},
		{"not an array", `"hello"`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if tt.json != "" {
				if err := json.Unmarshal([]byte(tt.json), &raw); err != nil {
					t.Fatalf("failed to parse: %v", err)
				}
			}
			got := extractTextFromToolCalls(raw)
			if got != tt.wantText {
				t.Errorf("got %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestNonStreamingSendsAuthorization(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var receivedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if receivedAuth != "Bearer test-key" {
		t.Errorf("upstream Authorization = %q, want %q", receivedAuth, "Bearer test-key")
	}
}

func TestStreamingSendsAuthorization(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var receivedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "stream-key", "", "")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if receivedAuth != "Bearer stream-key" {
		t.Errorf("streaming upstream Authorization = %q, want %q", receivedAuth, "Bearer stream-key")
	}
}

func TestClientAuthorizationNotForwarded(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var receivedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "server-key", "", "")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer client-leaked-key")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if receivedAuth != "Bearer server-key" {
		t.Errorf("upstream Authorization = %q, client key should not leak; want %q", receivedAuth, "Bearer server-key")
	}
}

func TestClientAuthorizationNotForwardedStreaming(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var receivedAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "server-stream-key", "", "")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer evil-client-key")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if receivedAuth != "Bearer server-stream-key" {
		t.Errorf("streaming upstream Authorization = %q, client key should not leak; want %q", receivedAuth, "Bearer server-stream-key")
	}
}

func TestEmptyAPIKeyReturnsLocal503(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called when API key is empty")
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "", "", "")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("empty API key should return 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not configured") {
		t.Errorf("body should mention missing key, got: %s", rec.Body.String())
	}
}

func TestAttributionHeadersSentWithAuthorization(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var receivedHeaders http.Header
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "https://example.com/referer", "TestApp")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := receivedHeaders.Get("Authorization"); got != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer test-key")
	}
	if got := receivedHeaders.Get("HTTP-Referer"); got != "https://example.com/referer" {
		t.Errorf("HTTP-Referer = %q, want %q", got, "https://example.com/referer")
	}
	if got := receivedHeaders.Get("X-OpenRouter-Title"); got != "TestApp" {
		t.Errorf("X-OpenRouter-Title = %q, want %q", got, "TestApp")
	}
}

func TestStatsShowsApiKeyPresent(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "")
	router := New(cfg, client)
	router.Stats.SetAPIKeyPresent(true)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	statsReq := httptest.NewRequest(http.MethodGet, "/debug/stats", nil)
	statsRec := httptest.NewRecorder()
	router.HandleDebugStats(statsRec, statsReq)

	if statsRec.Code != http.StatusOK {
		t.Fatalf("stats endpoint = %d", statsRec.Code)
	}

	var snap StatsSnapshot
	if err := json.NewDecoder(statsRec.Body).Decode(&snap); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if !snap.ApiKeyPresent {
		t.Error("api_key_present should be true")
	}
}

func TestApiKeyNotPresentWhenEmpty(t *testing.T) {
	s := NewStats()
	s.SetAPIKeyPresent(false)
	snap := s.Snapshot()
	if snap.ApiKeyPresent {
		t.Error("api_key_present should be false when API key is empty")
	}
}

func TestNoKeyInResponseHeaders(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
		})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "sk-or-secret-key-12345", "", "")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	for key, values := range rec.Header() {
		for _, v := range values {
			if strings.Contains(v, "sk-or-secret-key-12345") {
				t.Errorf("response header %q contains the API key: %s", key, v)
			}
			if strings.Contains(key, "sk-or-secret-key-12345") {
				t.Errorf("response header name %q contains the API key", key)
			}
		}
	}
}

func TestVersionEndpointDoesNotExposeKey(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	client := openrouter.NewClient("https://openrouter.ai/api/v1", "sk-or-secret-key-12345", "", "")
	router := New(cfg, client)

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	router.HandleVersion(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "sk-or-secret-key-12345") {
		t.Error("/version response should not contain the API key")
	}
	if strings.Contains(body, "Bearer") {
		t.Error("/version response should not contain Authorization bearer text")
	}
}

func TestHealthEndpointDoesNotExposeKey(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	client := openrouter.NewClient("https://openrouter.ai/api/v1", "sk-or-secret-key-12345", "", "")
	router := New(cfg, client)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.HandleHealth(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "sk-or-secret-key-12345") {
		t.Error("/health response should not contain the API key")
	}
}

func TestDebugStatsDoesNotExposeKey(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	client := openrouter.NewClient("https://openrouter.ai/api/v1", "sk-or-secret-key-12345", "", "")
	router := New(cfg, client)

	req := httptest.NewRequest(http.MethodGet, "/debug/stats", nil)
	rec := httptest.NewRecorder()
	router.HandleDebugStats(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "sk-or-secret-key-12345") {
		t.Error("/debug/stats should not contain the API key value")
	}
	if strings.Contains(body, "Bearer") {
		t.Error("/debug/stats should not contain Authorization bearer text")
	}
}
