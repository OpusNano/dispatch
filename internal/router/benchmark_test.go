package router

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func BenchmarkDebugRouteHandler(b *testing.B) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		b.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []map[string]interface{}{}})
	}))
	b.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function to sort an array and handle edge cases"}]}`
	bodyBytes := []byte(body)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(string(bodyBytes)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.HandleDebugRoute(rec, req)
	}
}

func BenchmarkChatCompletionsNonStream(b *testing.B) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		b.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.Copy(w, r.Body)
	}))
	b.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function to sort an array and handle edge cases"}],"stream":false}`

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.HandleChatCompletions(rec, req)
	}
}

func BenchmarkRequestRewrite(b *testing.B) {
	body := []byte(`{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}],"stream":true,"temperature":0.7,"max_tokens":4096}`)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		openrouter.RewriteRequest(body, "test-model", config.ProviderConfig{DataCollection: "deny"})
	}
}
