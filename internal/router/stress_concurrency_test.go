package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func TestConcurrentDebugRoute50(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function to sort an array"}]}`

	var wg sync.WaitGroup
	errCh := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.HandleDebugRoute(rec, req)
			if rec.Code != http.StatusOK {
				errCh <- fmt.Errorf("status %d: %s", rec.Code, rec.Body.String())
				return
			}
			var resp map[string]interface{}
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				errCh <- fmt.Errorf("decode: %w", err)
				return
			}
			if resp["level"] == nil {
				errCh <- fmt.Errorf("level missing")
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestConcurrentChatCompletions50(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}],"stream":false}`

	var wg sync.WaitGroup
	errCh := make(chan error, 50)
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.HandleChatCompletions(rec, req)
			if rec.Code != http.StatusOK {
				errCh <- fmt.Errorf("status %d: %s", rec.Code, rec.Body.String())
				return
			}
			if rec.Header().Get("X-Dispatch-Level") == "" {
				errCh <- fmt.Errorf("routing header missing")
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestConcurrentMixedStreamNonStream(t *testing.T) {
	router, _ := setupTest(t)
	streamBody := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	nonStreamBody := `{"model":"dispatch/auto","messages":[{"role":"user","content":"write a function"}],"stream":false}`

	var wg sync.WaitGroup
	errCh := make(chan error, 60)
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var body string
			if idx%2 == 0 {
				body = streamBody
			} else {
				body = nonStreamBody
			}
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.HandleChatCompletions(rec, req)
			if rec.Code != http.StatusOK {
				errCh <- fmt.Errorf("request %d: status %d: %s", idx, rec.Code, rec.Body.String())
				return
			}
			if rec.Header().Get("X-Dispatch-Level") == "" {
				errCh <- fmt.Errorf("request %d: routing header missing", idx)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestConcurrentRequestIDsUnique(t *testing.T) {
	router, _ := setupTest(t)
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`

	const N = 100
	ids := make([]string, N)
	var wg sync.WaitGroup
	errCh := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.HandleChatCompletions(rec, req)
			id := rec.Header().Get("X-Dispatch-Request-Id")
			if id == "" {
				errCh <- fmt.Errorf("empty request ID at index %d", idx)
				return
			}
			ids[idx] = id
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}

	seen := make(map[string]bool, N)
	for i, id := range ids {
		if id == "" {
			t.Errorf("request %d: empty ID", i)
			continue
		}
		if seen[id] {
			t.Errorf("duplicate request ID: %s", id)
		}
		seen[id] = true
	}
}

func TestConcurrentDebugRouteNoUpstreamCalls(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	var upstreamCalls int
	var callMu sync.Mutex
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callMu.Lock()
		upstreamCalls++
		callMu.Unlock()
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}]}`

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/debug/route", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.HandleDebugRoute(rec, req)
		}()
	}
	wg.Wait()

	callMu.Lock()
	if upstreamCalls > 0 {
		t.Errorf("upstream called %d times during concurrent debug/route", upstreamCalls)
	}
	callMu.Unlock()
}

func TestConcurrentHealthAndVersion(t *testing.T) {
	router, _ := setupTest(t)

	var wg sync.WaitGroup
	errCh := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			if idx%2 == 0 {
				req := httptest.NewRequest(http.MethodGet, "/health", nil)
				router.HandleHealth(rec, req)
				if rec.Code != http.StatusOK {
					errCh <- fmt.Errorf("health status %d", rec.Code)
				}
			} else {
				req := httptest.NewRequest(http.MethodGet, "/version", nil)
				router.HandleVersion(rec, req)
				if rec.Code != http.StatusOK {
					errCh <- fmt.Errorf("version status %d", rec.Code)
				}
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

func TestConcurrentUpstreamSlowResponses(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var parsed map[string]interface{}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &parsed)
		if parsed["stream"] == true {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\n"))
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
			w.Write([]byte("data: [DONE]\n\n"))
			if fl, ok := w.(http.Flusher); ok {
				fl.Flush()
			}
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []map[string]interface{}{{"message": map[string]string{"content": "ok"}}},
			})
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	bodies := []string{
		`{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`,
		`{"model":"dispatch/auto","messages":[{"role":"user","content":"write code"}],"stream":true}`,
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := bodies[idx%2]
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.HandleChatCompletions(rec, req)
			if rec.Code != http.StatusOK {
				errCh <- fmt.Errorf("req %d: status %d", idx, rec.Code)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}
