package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func TestExactMaxBodySizeAccepted(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Server.MaxBodySize = 200

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []map[string]interface{}{}})
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	prefix := `{"model":"dispatch/auto","messages":[{"role":"user","content":"`
	suffix := `"}],"stream":false}`
	paddingLen := int(cfg.Server.MaxBodySize) - len(prefix) - len(suffix)
	if paddingLen < 0 {
		t.Fatal("max body size too small for test")
	}
	exactBody := prefix + strings.Repeat("x", paddingLen) + suffix
	if int64(len(exactBody)) != cfg.Server.MaxBodySize {
		t.Fatalf("body length %d != max %d", len(exactBody), cfg.Server.MaxBodySize)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(exactBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code == http.StatusRequestEntityTooLarge {
		t.Errorf("exact max body size should be accepted, got 413")
	}
}

func TestMaxBodySizePlusOneRejected(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Server.MaxBodySize = 200

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called for oversize body")
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	prefix := `{"model":"dispatch/auto","messages":[{"role":"user","content":"`
	suffix := `"}],"stream":false}`
	paddingLen := int(cfg.Server.MaxBodySize) - len(prefix) - len(suffix) + 1
	if paddingLen < 0 {
		t.Fatal("max body size too small for test")
	}
	oversizeBody := prefix + strings.Repeat("x", paddingLen) + suffix

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(oversizeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize body should be rejected with 413, got %d", rec.Code)
	}
}

func TestBodyValidJSONNoModel(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on missing model: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("missing model should still work (auto-classify), got %d", rec.Code)
	}
}

func TestBodyModelAsNumber(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":42,"messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on model as number: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("model as number should not crash, got %d", rec.Code)
	}
}

func TestBodyModelAsObject(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":{"nested":true},"messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on model as object: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("model as object should not crash, got %d", rec.Code)
	}
}

func TestBodyModelAsNull(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":null,"messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on model as null: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("model as null should not crash, got %d", rec.Code)
	}
}

func TestBodyMessagesAsObject(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":{"role":"user","content":"hi"},"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on messages as object: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("messages as object should not crash, got %d", rec.Code)
	}
}

func TestBodyMessagesAsNull(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":null,"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on messages as null: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("messages as null should not crash, got %d", rec.Code)
	}
}

func TestBodyMessagesAsNumber(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":123,"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on messages as number: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("messages as number should not crash, got %d", rec.Code)
	}
}

func TestBodyMessagesAsString(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":"hello","stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on messages as string: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("messages as string should not crash, got %d", rec.Code)
	}
}

func TestBodyHugeToolsArray(t *testing.T) {
	router, _ := setupTest(t)
	var toolsJSON strings.Builder
	toolsJSON.WriteString(`{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"tools":[`)
	for i := 0; i < 1000; i++ {
		if i > 0 {
			toolsJSON.WriteString(",")
		}
		toolsJSON.WriteString(`{"type":"function","function":{"name":"tool_`)
		toolsJSON.WriteString(fmt.Sprintf("%d", i))
		toolsJSON.WriteString(`","description":"test","parameters":{"type":"object","properties":{}}}}`)
	}
	toolsJSON.WriteString(`],"stream":false}`)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(toolsJSON.String()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on huge tools array: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("huge tools array should not crash, got %d", rec.Code)
	}
	level := rec.Header().Get("X-Dispatch-Level")
	if level == "" {
		t.Error("should classify with tools present")
	}
}

func TestBodyWeirdResponseFormat(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_schema","json_schema":{"name":"foo","schema":{"type":"object"}}},"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on weird response_format: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("weird response_format should not crash, got %d", rec.Code)
	}
}

func TestBodyResponseFormatAsNumber(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"response_format":42,"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on response_format as number: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("response_format as number should not crash, got %d", rec.Code)
	}
}

func TestBodyStreamAsObject(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":{"enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on stream as object: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("stream as object should not crash, got %d", rec.Code)
	}
}

func TestBodyStreamAsNumber(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":1}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on stream as number: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("stream as number should not crash, got %d", rec.Code)
	}
}

func TestBodyExtremelyDeepNesting(t *testing.T) {
	router, _ := setupTest(t)
	var sb strings.Builder
	sb.WriteString(`{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"extra":`)
	for i := 0; i < 100; i++ {
		sb.WriteString(`{"nested":`)
	}
	sb.WriteString(`true`)
	for i := 0; i < 100; i++ {
		sb.WriteString(`}`)
	}
	sb.WriteString(`}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(sb.String()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on deep nesting: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("deep nesting should not crash, got %d", rec.Code)
	}
}

func TestDebugRouteBodyNoModel(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleDebugRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("debug route without model should work, got %d", rec.Code)
	}
}

func TestDebugRouteModelAsNumber(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":42,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on debug route model as number: %v", r)
		}
	}()
	router.HandleDebugRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("debug route model as number should not crash, got %d", rec.Code)
	}
}
