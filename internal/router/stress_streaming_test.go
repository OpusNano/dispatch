package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func TestStreamingManyTinyChunks(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\n"))
			if fl != nil {
				fl.Flush()
			}
		}
		w.Write([]byte("data: [DONE]\n\n"))
		if fl != nil {
			fl.Flush()
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	respBody := rec.Body.String()
	if strings.Count(respBody, "data: ") != 101 {
		t.Errorf("expected 101 data lines, got %d", strings.Count(respBody, "data: "))
	}
	if !strings.Contains(respBody, "[DONE]") {
		t.Error("DONE marker missing")
	}
}

func TestStreamingSlowChunks(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		chunks := []string{
			"data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n",
			"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n",
			"data: [DONE]\n\n",
		}
		for _, c := range chunks {
			w.Write([]byte(c))
			if fl != nil {
				fl.Flush()
			}
			time.Sleep(20 * time.Millisecond)
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "hello") {
		t.Error("first chunk missing")
	}
	if !strings.Contains(respBody, " world") {
		t.Error("second chunk missing")
	}
	if !strings.Contains(respBody, "[DONE]") {
		t.Error("DONE missing")
	}
}

func TestStreamingCommentOnlyChunks(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		for i := 0; i < 5; i++ {
			w.Write([]byte(": keepalive comment\n\n"))
			if fl != nil {
				fl.Flush()
			}
		}
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
		if fl != nil {
			fl.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		if fl != nil {
			fl.Flush()
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	respBody := rec.Body.String()
	if strings.Count(respBody, "keepalive comment") != 5 {
		t.Errorf("expected 5 comment chunks, got %d", strings.Count(respBody, "keepalive comment"))
	}
	if !strings.Contains(respBody, "data: [DONE]") {
		t.Error("DONE missing")
	}
}

func TestStreamingNoDoneMarker(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"no done\"}}]}\n\n"))
		if fl != nil {
			fl.Flush()
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on stream without DONE: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "no done") {
		t.Error("content missing")
	}
	if strings.Contains(respBody, "[DONE]") {
		t.Error("should not contain DONE marker")
	}
}

func TestStreamingUpstreamClosesMidChunk(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial"))
		if fl != nil {
			fl.Flush()
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on mid-chunk close: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "partial") {
		t.Error("partial content should be passed through")
	}
}

func TestStreamingInvalidJSONInSSE(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		w.Write([]byte("data: {not valid json at all}\n\n"))
		if fl != nil {
			fl.Flush()
		}
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		if fl != nil {
			fl.Flush()
		}
		w.Write([]byte("data: [DONE]\n\n"))
		if fl != nil {
			fl.Flush()
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on invalid JSON in SSE: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	respBody := rec.Body.String()
	if !strings.Contains(respBody, "not valid json") {
		t.Error("invalid JSON chunk should be passed through verbatim")
	}
	if !strings.Contains(respBody, "ok") {
		t.Error("valid chunk after invalid one should still arrive")
	}
}

func TestStreamingEmptyUpstream(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic on empty stream: %v", r)
		}
	}()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body, got %d bytes", rec.Body.Len())
	}
}

func TestStreamingUpstreamErrorStatus(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("data: {\"error\":\"rate limited\"}\n\n"))
		if fl, ok := w.(http.Flusher); ok {
			fl.Flush()
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("should pass through 429, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "rate limited") {
		t.Error("error body should be passed through")
	}
}
