package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func TestUpstreamNoContentType(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
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

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Error("body should pass through even without Content-Type")
	}
}

func TestUpstreamHugeHeaderValues(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	hugeValue := strings.Repeat("x", 8000)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Huge-Header", hugeValue)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[]}`))
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

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	h := rec.Header().Get("X-Huge-Header")
	if len(h) != 8000 {
		t.Errorf("huge header length = %d, want 8000", len(h))
	}
}

func TestUpstreamEmptyBody200(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
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

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body, got %d bytes", rec.Body.Len())
	}
}

func TestUpstream204NoContent(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
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

	if rec.Code != http.StatusNoContent {
		t.Errorf("should pass through 204, got %d", rec.Code)
	}
}

func TestUpstream304NotModified(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
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

	if rec.Code != http.StatusNotModified {
		t.Errorf("should pass through 304, got %d", rec.Code)
	}
}

func TestUpstreamChunkedNonStreamResponse(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"`))
		w.Write([]byte(`hello chunked`))
		w.Write([]byte(`"}}]}`))
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

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hello chunked") {
		t.Error("chunked body should be reassembled correctly")
	}
}

func TestUpstreamMultipleHeadersSameName(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Multi", "one")
		w.Header().Add("X-Multi", "two")
		w.Header().Add("X-Multi", "three")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[]}`))
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

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	values := rec.Header().Values("X-Multi")
	if len(values) != 3 {
		t.Errorf("expected 3 X-Multi headers, got %d: %v", len(values), values)
	}
}
