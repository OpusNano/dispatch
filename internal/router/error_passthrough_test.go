package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"dispatch/internal/config"
	"dispatch/internal/openrouter"
)

func TestNonStream429OpenRouterErrorPassthrough(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true
	cfg.Debug.RequestIndexEnabled = true
	cfg.Debug.LogMetadata = true

	errorBody := `{"error":{"message":"Provider returned error","code":429,"metadata":{"raw":"deepseek/deepseek-v4-flash is temporarily rate-limited upstream. Please retry shortly, or add your own key to accumulate your rate limits","provider_name":"Baidu","is_byok":false}}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(errorBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_429", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Provider returned error") {
		t.Error("body should contain original error message")
	}
	if !strings.Contains(rec.Body.String(), "Baidu") {
		t.Error("body should contain provider_name")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %s, want application/json", ct)
	}

	if rec.Header().Get("X-Dispatch-Upstream-Status") != "429" {
		t.Errorf("X-Dispatch-Upstream-Status = %s", rec.Header().Get("X-Dispatch-Upstream-Status"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Error-Code") != "429" {
		t.Errorf("X-Dispatch-Upstream-Error-Code = %s", rec.Header().Get("X-Dispatch-Upstream-Error-Code"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Provider") != "Baidu" {
		t.Errorf("X-Dispatch-Upstream-Provider = %s", rec.Header().Get("X-Dispatch-Upstream-Provider"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Is-BYOK") != "false" {
		t.Errorf("X-Dispatch-Upstream-Is-BYOK = %s", rec.Header().Get("X-Dispatch-Upstream-Is-BYOK"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Retry-After") != "30" {
		t.Errorf("X-Dispatch-Upstream-Retry-After = %q, want 30", rec.Header().Get("X-Dispatch-Upstream-Retry-After"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Retryable") != "true" {
		t.Errorf("X-Dispatch-Upstream-Retryable = %s, want true", rec.Header().Get("X-Dispatch-Upstream-Retryable"))
	}

	snap := router.Stats.Snapshot()
	if snap.Upstream429Total != 1 {
		t.Errorf("upstream_429_total = %d, want 1", snap.Upstream429Total)
	}
	if snap.UpstreamErrorsTotal != 1 {
		t.Errorf("upstream_errors_total = %d, want 1", snap.UpstreamErrorsTotal)
	}
	if snap.UpstreamRateLimits != 1 {
		t.Errorf("upstream_rate_limits_total = %d, want 1", snap.UpstreamRateLimits)
	}
	if snap.ByUpstreamProvider["Baidu"] != 1 {
		t.Errorf("by_upstream_provider[Baidu] = %d, want 1", snap.ByUpstreamProvider["Baidu"])
	}
	if snap.ByStatus[429] != 1 {
		t.Errorf("by_status[429] = %d, want 1", snap.ByStatus[429])
	}

	requestID := rec.Header().Get("X-Dispatch-Request-Id")
	meta, ok := router.RequestIndex.Lookup(requestID)
	if !ok {
		t.Fatal("request not in index")
	}
	if meta.UpstreamErrorCode != 429 {
		t.Errorf("meta.UpstreamErrorCode = %d, want 429", meta.UpstreamErrorCode)
	}
	if meta.UpstreamProvider != "Baidu" {
		t.Errorf("meta.UpstreamProvider = %s, want Baidu", meta.UpstreamProvider)
	}
	if meta.UpstreamRetryable != "true" {
		t.Errorf("meta.UpstreamRetryable = %s, want true", meta.UpstreamRetryable)
	}
	if !strings.Contains(meta.UpstreamRawTruncated, "rate-limited") {
		t.Errorf("meta.UpstreamRawTruncated should contain 'rate-limited', got %q", meta.UpstreamRawTruncated)
	}
	if meta.Status != 429 {
		t.Errorf("meta.Status = %d, want 429", meta.Status)
	}
}

func TestStreamPreToken429ErrorKeepsContentTypeJSON(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true
	errorBody := `{"error":{"message":"rate limited","code":429}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(errorBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_STR429", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json (NOT text/event-stream)", ct)
	}
	if strings.Contains(ct, "text/event-stream") {
		t.Error("Content-Type must NOT be text/event-stream for pre-token streaming error")
	}

	if !strings.EqualFold(strings.TrimSpace(rec.Body.String()), strings.TrimSpace(errorBody)) {
		t.Errorf("body should be byte-identical to upstream error body\ngot:  %s\nwant: %s", rec.Body.String(), errorBody)
	}

	if rec.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Retry-After") != "60" {
		t.Errorf("X-Dispatch-Upstream-Retry-After = %q, want 60", rec.Header().Get("X-Dispatch-Upstream-Retry-After"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Retryable") != "true" {
		t.Errorf("X-Dispatch-Upstream-Retryable = %s, want true", rec.Header().Get("X-Dispatch-Upstream-Retryable"))
	}
}

func TestAllErrorStatusesPassthrough(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		retryable  string
	}{
		{"400 invalid_request", http.StatusBadRequest, `{"error":{"message":"bad request","code":400}}`, "false"},
		{"401 authentication", http.StatusUnauthorized, `{"error":{"message":"invalid key","code":401}}`, "false"},
		{"402 payment_required", http.StatusPaymentRequired, `{"error":{"message":"no credits","code":402}}`, "false"},
		{"403 permission_denied", http.StatusForbidden, `{"error":{"message":"forbidden","code":403}}`, "false"},
		{"408 timeout", http.StatusRequestTimeout, `{"error":{"message":"timeout","code":408}}`, "true"},
		{"502 provider_unavailable", http.StatusBadGateway, `{"error":{"message":"bad gateway","code":502}}`, "true"},
		{"503 provider_overloaded", http.StatusServiceUnavailable, `{"error":{"message":"unavailable","code":503}}`, "true"},
		{"504 timeout", http.StatusGatewayTimeout, `{"error":{"message":"gateway timeout","code":504}}`, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := config.DefaultConfig()
			cfg.Debug.SetResponseHeaders = true
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			t.Cleanup(upstream.Close)
			cfg.OpenRouter.BaseURL = upstream.URL
			os.Setenv("TEST_OR_"+strconv.Itoa(tt.statusCode), "test-key")
			client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
			router := New(cfg, client)

			body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.HandleChatCompletions(rec, req)

			if rec.Code != tt.statusCode {
				t.Errorf("status = %d, want %d", rec.Code, tt.statusCode)
			}
			if !strings.Contains(rec.Body.String(), "error") {
				t.Error("error body should be preserved")
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
				t.Errorf("Content-Type = %s, want application/json", ct)
			}
			if rec.Header().Get("X-Dispatch-Upstream-Status") != strconv.Itoa(tt.statusCode) {
				t.Errorf("X-Dispatch-Upstream-Status = %s, want %d", rec.Header().Get("X-Dispatch-Upstream-Status"), tt.statusCode)
			}
			if rec.Header().Get("X-Dispatch-Upstream-Retryable") != tt.retryable {
				t.Errorf("X-Dispatch-Upstream-Retryable = %s, want %s", rec.Header().Get("X-Dispatch-Upstream-Retryable"), tt.retryable)
			}
		})
	}
}

func TestNonStream200EmbeddedError(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true
	cfg.Debug.RequestIndexEnabled = true

	embeddedBody := `{"id":"chat-123","choices":[{"finish_reason":"error","error":{"code":429,"message":"provider error","metadata":{"error_type":"rate_limit_exceeded","provider_code":"RATE_LIMIT","provider_name":"TestProvider"}}}],"model":"test-model"}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(embeddedBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_EMBEDDED", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (embedded error is still 200)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "rate_limit_exceeded") {
		t.Error("body should contain embedded error details")
	}
	if !strings.Contains(rec.Body.String(), "TestProvider") {
		t.Error("body should contain provider name")
	}

	if rec.Header().Get("X-Dispatch-Upstream-Error-Type") != "rate_limit_exceeded" {
		t.Errorf("X-Dispatch-Upstream-Error-Type = %s, want rate_limit_exceeded", rec.Header().Get("X-Dispatch-Upstream-Error-Type"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Provider-Code") != "RATE_LIMIT" {
		t.Errorf("X-Dispatch-Upstream-Provider-Code = %s, want RATE_LIMIT", rec.Header().Get("X-Dispatch-Upstream-Provider-Code"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Provider") != "TestProvider" {
		t.Errorf("X-Dispatch-Upstream-Provider = %s, want TestProvider", rec.Header().Get("X-Dispatch-Upstream-Provider"))
	}

	snap := router.Stats.Snapshot()
	if snap.UpstreamEmbeddedTotal != 1 {
		t.Errorf("upstream_embedded_errors_total = %d, want 1", snap.UpstreamEmbeddedTotal)
	}
	if snap.UpstreamErrorsTotal != 1 {
		t.Errorf("upstream_errors_total = %d, want 1", snap.UpstreamErrorsTotal)
	}

	requestID := rec.Header().Get("X-Dispatch-Request-Id")
	meta, ok := router.RequestIndex.Lookup(requestID)
	if !ok {
		t.Fatal("request not in index")
	}
	if !meta.EmbeddedError {
		t.Error("meta.EmbeddedError should be true")
	}
	if meta.UpstreamErrorType != "rate_limit_exceeded" {
		t.Errorf("meta.UpstreamErrorType = %s, want rate_limit_exceeded", meta.UpstreamErrorType)
	}
	if meta.UpstreamProviderCode != "RATE_LIMIT" {
		t.Errorf("meta.UpstreamProviderCode = %s, want RATE_LIMIT", meta.UpstreamProviderCode)
	}
	if meta.Status != 200 {
		t.Errorf("meta.Status = %d, want 200 (embedded error)", meta.Status)
	}
}

func TestNoPromptTextInErrorDiagnostics(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true
	cfg.Debug.RequestIndexEnabled = true

	errorBody := `{"error":{"message":"Provider returned error","code":429,"metadata":{"raw":"rate limited","provider_name":"Baidu"}}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(errorBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_NOPROMPT", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	prompt := `{"model":"dispatch/auto","messages":[{"role":"user","content":"this is a secret prompt with my-api-key sk-secret123"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(prompt))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	requestID := rec.Header().Get("X-Dispatch-Request-Id")
	meta, ok := router.RequestIndex.Lookup(requestID)
	if !ok {
		t.Fatal("request not in index")
	}
	if meta.LatestUserHash == "" || strings.Contains(meta.LatestUserHash, "secret") {
		t.Error("user content should be hashed, not stored in plain text")
	}
	if meta.FrameHash == "" || strings.Contains(meta.FrameHash, "secret") {
		t.Error("frame should be hashed, not stored in plain text")
	}

	for _, header := range []string{"X-Dispatch-Upstream-Raw-Error"} {
		if rec.Header().Get(header) != "" {
			t.Errorf("%s should not be present", header)
		}
	}
	for _, header := range []string{
		"X-Dispatch-Upstream-Status",
		"X-Dispatch-Upstream-Error-Code",
		"X-Dispatch-Upstream-Error-Type",
		"X-Dispatch-Upstream-Provider",
	} {
		if hval := rec.Header().Get(header); hval != "" {
			if strings.Contains(strings.ToLower(hval), "secret") {
				t.Errorf("%s header contains prompt text: %s", header, hval)
			}
		}
	}
}

func TestNoAuthHeaderInDiagnostics(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true

	errorBody := `{"error":{"message":"unauthorized","code":401}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(errorBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_NOAUTH2", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	for key := range rec.Header() {
		val := rec.Header().Get(key)
		if strings.Contains(strings.ToLower(val), "test-key") {
			t.Errorf("header %s contains API key: %s", key, val)
		}
		if strings.Contains(strings.ToLower(val), "bearer") {
			t.Errorf("header %s contains auth: %s", key, val)
		}
	}
}

func TestSuccessfulSSERemainsPassthrough(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true

	chunks := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\" World\"}}]}\n\n",
		"data: [DONE]\n\n",
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fl, _ := w.(http.Flusher)
		for _, c := range chunks {
			w.Write([]byte(c))
			if fl != nil {
				fl.Flush()
			}
		}
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_SSE", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"test"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %s, want text/event-stream", rec.Header().Get("Content-Type"))
	}
	bodyStr := rec.Body.String()
	if !strings.Contains(bodyStr, "data: [DONE]") {
		t.Error("DONE marker missing")
	}
	if !strings.Contains(bodyStr, "Hello") || !strings.Contains(bodyStr, "World") {
		t.Error("SSE content modified")
	}
}

func TestUpstreamConnectionErrorNotConvertedToRetryable(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_CONN", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	upstream.Close()

	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("connection error should return 502, got %d", rec.Code)
	}

	for _, header := range []string{
		"X-Dispatch-Upstream-Status",
		"X-Dispatch-Upstream-Retryable",
	} {
		if rec.Header().Get(header) != "" {
			t.Errorf("connection error should not have %s header", header)
		}
	}
}

func TestRawErrorTextTruncatedInMeta(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.RequestIndexEnabled = true

	longRaw := strings.Repeat("error detail ", 100)

	errorBody := `{"error":{"message":"error","code":502,"metadata":{"raw":"` + longRaw + `","provider_name":"Test"}}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(errorBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_TRUNC", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	requestID := rec.Header().Get("X-Dispatch-Request-Id")
	meta, ok := router.RequestIndex.Lookup(requestID)
	if !ok {
		t.Fatal("request not in index")
	}
	if len(meta.UpstreamRawTruncated) > 300 {
		t.Errorf("raw truncated should be max 300 chars, got %d", len(meta.UpstreamRawTruncated))
	}
	if !strings.Contains(meta.UpstreamRawTruncated, "error detail") {
		t.Error("truncated raw should contain the prefix")
	}
}

func TestErrorMetaRetryableClassification(t *testing.T) {
	tests := []struct {
		status    int
		errorType string
		retryable string
	}{
		{400, "", "false"},
		{401, "", "false"},
		{402, "", "false"},
		{403, "", "false"},
		{404, "", "false"},
		{408, "", "true"},
		{409, "", "unknown"},
		{429, "", "true"},
		{500, "", "unknown"},
		{502, "", "true"},
		{503, "", "true"},
		{504, "", "true"},
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.status), func(t *testing.T) {
			errorJSON := `{"error":{"code":` + strconv.Itoa(tt.status) + `,"message":"test"}}`
			resp := &http.Response{
				StatusCode: tt.status,
				Header:     make(http.Header),
			}
			meta := openrouter.ExtractErrorMeta(resp, []byte(errorJSON))
			if meta == nil {
				t.Fatal("meta should not be nil")
			}
			if meta.Retryable != tt.retryable {
				t.Errorf("status %d: retryable = %s, want %s", tt.status, meta.Retryable, tt.retryable)
			}
		})
	}
}

func TestRetryAfterPreservation(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":{"code":503,"message":"overloaded"}}`))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_RETRY", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Header().Get("Retry-After") != "120" {
		t.Errorf("Retry-After = %q, want 120 (should pass through)", rec.Header().Get("Retry-After"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Retry-After") != "120" {
		t.Errorf("X-Dispatch-Upstream-Retry-After = %q, want 120", rec.Header().Get("X-Dispatch-Upstream-Retry-After"))
	}
}

func TestExtractErrorMetaNilResponse(t *testing.T) {
	if meta := openrouter.ExtractErrorMeta(nil, nil); meta != nil {
		t.Error("nil response should return nil meta")
	}
}

func TestExtractErrorMetaNoErrorInBody(t *testing.T) {
	resp := &http.Response{StatusCode: 200, Header: make(http.Header)}
	resp.Header.Set("Content-Type", "application/json")
	meta := openrouter.ExtractErrorMeta(resp, []byte(`{"choices":[{"message":{"content":"hello"}}]}`))
	if meta != nil {
		t.Error("no error in body should return nil")
	}
}

func TestExtractErrorMetaMalformedJSON(t *testing.T) {
	resp := &http.Response{StatusCode: 400, Header: make(http.Header)}
	meta := openrouter.ExtractErrorMeta(resp, []byte(`not json`))
	if meta == nil {
		t.Fatal("should still create meta for status >= 400")
	}
	if meta.ErrorParsed {
		t.Error("malformed JSON should not be parsed")
	}
	if meta.HTTPStatus != 400 {
		t.Errorf("HTTPStatus = %d, want 400", meta.HTTPStatus)
	}
}

func TestDiagnosticHeadersAddedWithoutChangingBody(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true

	originalBody := `{"error":{"code":429,"message":"Too Many Requests","metadata":{"error_type":"rate_limit_exceeded","provider_name":"ExampleProvider"}}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(originalBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_DIAG", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	var respBody map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &respBody)
	errorObj, _ := respBody["error"].(map[string]interface{})
	if errorObj == nil {
		t.Fatal("error object missing from response body")
	}
	if errorObj["code"] != float64(429) {
		t.Errorf("error.code = %v, want 429", errorObj["code"])
	}
	if !strings.Contains(rec.Body.String(), "rate_limit_exceeded") {
		t.Error("error_type should be in body")
	}

	if rec.Header().Get("X-Dispatch-Upstream-Error-Type") != "rate_limit_exceeded" {
		t.Errorf("X-Dispatch-Upstream-Error-Type = %s", rec.Header().Get("X-Dispatch-Upstream-Error-Type"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Provider") != "ExampleProvider" {
		t.Errorf("X-Dispatch-Upstream-Provider = %s", rec.Header().Get("X-Dispatch-Upstream-Provider"))
	}
}

func TestUpstreamErrorWithAllMetadataFields(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true
	cfg.Debug.RequestIndexEnabled = true

	errorBody := `{"error":{"code":503,"message":"Provider overloaded","metadata":{"error_type":"provider_overloaded","provider_code":"OVERLOAD_429","provider_name":"TestProviderInc","is_byok":true,"model_slug":"test/model-v1","raw":"Upstream returned 429 from TestProviderInc"}}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "15")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(errorBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_FULL", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if rec.Header().Get("X-Dispatch-Upstream-Error-Type") != "provider_overloaded" {
		t.Errorf("error_type = %s", rec.Header().Get("X-Dispatch-Upstream-Error-Type"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Provider-Code") != "OVERLOAD_429" {
		t.Errorf("provider_code = %s", rec.Header().Get("X-Dispatch-Upstream-Provider-Code"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Provider") != "TestProviderInc" {
		t.Errorf("provider = %s", rec.Header().Get("X-Dispatch-Upstream-Provider"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Is-BYOK") != "true" {
		t.Errorf("is_byok = %s", rec.Header().Get("X-Dispatch-Upstream-Is-BYOK"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Retry-After") != "15" {
		t.Errorf("retry_after = %s", rec.Header().Get("X-Dispatch-Upstream-Retry-After"))
	}
	if rec.Header().Get("X-Dispatch-Upstream-Retryable") != "true" {
		t.Errorf("retryable = %s", rec.Header().Get("X-Dispatch-Upstream-Retryable"))
	}

	requestID := rec.Header().Get("X-Dispatch-Request-Id")
	meta, ok := router.RequestIndex.Lookup(requestID)
	if !ok {
		t.Fatal("request not in index")
	}
	if meta.UpstreamErrorType != "provider_overloaded" {
		t.Errorf("meta error_type = %s", meta.UpstreamErrorType)
	}
	if meta.UpstreamProviderCode != "OVERLOAD_429" {
		t.Errorf("meta provider_code = %s", meta.UpstreamProviderCode)
	}
	if meta.UpstreamIsBYOK != "true" {
		t.Errorf("meta is_byok = %s", meta.UpstreamIsBYOK)
	}
	if meta.UpstreamRetryAfter != "15" {
		t.Errorf("meta retry_after = %s", meta.UpstreamRetryAfter)
	}
	if meta.Status != 503 {
		t.Errorf("meta status = %d, want 503", meta.Status)
	}

	snap := router.Stats.Snapshot()
	if snap.Upstream503Total != 1 {
		t.Errorf("upstream_503_total = %d", snap.Upstream503Total)
	}
	if snap.ByUpstreamErrorType["provider_overloaded"] != 1 {
		t.Errorf("by_upstream_error_type[provider_overloaded] = %d", snap.ByUpstreamErrorType["provider_overloaded"])
	}
	if snap.ByUpstreamProvider["TestProviderInc"] != 1 {
		t.Errorf("by_upstream_provider[TestProviderInc] = %d", snap.ByUpstreamProvider["TestProviderInc"])
	}
	if snap.ByUpstreamProviderCode["OVERLOAD_429"] != 1 {
		t.Errorf("by_upstream_provider_code[OVERLOAD_429] = %d", snap.ByUpstreamProviderCode["OVERLOAD_429"])
	}
}

func TestRequestLookupShowsErrorDiagnostics(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.RequestIndexEnabled = true

	errorBody := `{"error":{"code":502,"message":"Bad Gateway","metadata":{"provider_name":"DownProvider","error_type":"provider_unavailable"}}}`
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(errorBody))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_LOOKUP", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	requestID := rec.Header().Get("X-Dispatch-Request-Id")

	lookupReq := httptest.NewRequest(http.MethodGet, "/debug/request?id="+requestID, nil)
	lookupRec := httptest.NewRecorder()
	router.HandleDebugRequestLookup(lookupRec, lookupReq)

	if lookupRec.Code != http.StatusOK {
		t.Fatalf("lookup status = %d", lookupRec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(lookupRec.Body).Decode(&resp)

	if resp["upstream_provider"] != "DownProvider" {
		t.Errorf("upstream_provider = %v", resp["upstream_provider"])
	}
	if resp["upstream_error_type"] != "provider_unavailable" {
		t.Errorf("upstream_error_type = %v", resp["upstream_error_type"])
	}
	if resp["status"] != float64(502) {
		t.Errorf("status = %v", resp["status"])
	}
	if resp["stream_error_observed"] != nil {
		t.Log("stream_error_observed field not present (expected — mid-stream SSE observation not implemented)")
	}
	if resp["latest_user_text"] != nil {
		t.Error("response should NOT contain prompt text")
	}
}

func TestConfigCheckWarnsOnEmptyAttribution(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.OpenRouter.HTTPReferer = ""
	cfg.OpenRouter.SiteTitle = ""

	var buf strings.Builder
	cfg.PrintConfig(&buf)

	_ = buf.String()
}

func TestEmptyBodyUpstreamReturnsEmptyBody(t *testing.T) {
	cfg, _ := config.DefaultConfig()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_EMPTY", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rec.Code)
	}
}

func TestErrorExtractionWithErrorTypeRetryable(t *testing.T) {
	tests := []struct {
		errorType string
		retryable string
	}{
		{"rate_limit_exceeded", "true"},
		{"provider_overloaded", "true"},
		{"provider_unavailable", "true"},
		{"timeout", "true"},
		{"invalid_request", "false"},
		{"authentication", "false"},
		{"permission_denied", "false"},
		{"payment_required", "false"},
		{"content_policy_violation", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.errorType, func(t *testing.T) {
			errorBody := `{"error":{"code":500,"message":"test","metadata":{"error_type":"` + tt.errorType + `"}}}`
			resp := &http.Response{StatusCode: 500, Header: make(http.Header)}
			meta := openrouter.ExtractErrorMeta(resp, []byte(errorBody))
			if meta == nil {
				t.Fatal("meta nil")
			}
			if meta.Retryable != tt.retryable {
				t.Errorf("%s: retryable = %s, want %s", tt.errorType, meta.Retryable, tt.retryable)
			}
		})
	}
}

func TestStreamTrueUpstream429KeepsStatus(t *testing.T) {
	cfg, _ := config.DefaultConfig()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"code":429,"message":"rate limited"}}`))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_STRSTAT", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("stream=true with 429 upstream: status = %d, want 429", rec.Code)
	}
}

func TestDispatchDoesNotRetryInternally(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"code":429,"message":"rate limited"}}`))
	}))
	t.Cleanup(upstream.Close)
	cfg.OpenRouter.BaseURL = upstream.URL
	os.Setenv("TEST_OR_NORETRY", "test-key")
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if callCount != 1 {
		t.Errorf("upstream called %d times, want 1 (no internal retries)", callCount)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 (should pass through, not convert to 502)", rec.Code)
	}
}

func TestUpstreamErrorDiagnosticBodyParseCap(t *testing.T) {
	resp := &http.Response{
		StatusCode: 400,
		Header:     make(http.Header),
	}
	resp.Header.Set("Content-Type", "application/json")

	body := []byte(`{"error":{"code":400,"message":"bad"}}` + strings.Repeat(" ", 200000))
	meta := openrouter.ExtractErrorMeta(resp, body)
	if meta == nil {
		t.Fatal("meta nil")
	}
	if meta.DiagnosticTrunc {
		t.Log("diagnostic truncation flag set for oversized body (expected)")
	}
	if !meta.ErrorParsed {
		t.Error("should still parse error from oversized body")
	}
}

func TestMissingAPIKeyDoesNotSetUpstreamHeaders(t *testing.T) {
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

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	for _, h := range []string{
		"X-Dispatch-Upstream-Status",
		"X-Dispatch-Upstream-Error-Code",
		"X-Dispatch-Upstream-Error-Type",
		"X-Dispatch-Upstream-Provider",
		"X-Dispatch-Upstream-Retryable",
	} {
		if v := rec.Header().Get(h); v != "" {
			t.Errorf("local error should not have %s = %q", h, v)
		}
	}
	snap := router.Stats.Snapshot()
	if snap.UpstreamErrorsTotal != 0 {
		t.Errorf("upstream_errors_total = %d, want 0 (missing API key is a local error)", snap.UpstreamErrorsTotal)
	}
}

func TestInvalidJSONDoesNotSetUpstreamHeaders(t *testing.T) {
	router, _ := setupTest(t)
	body := `{invalid json not parseable}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	for _, h := range []string{
		"X-Dispatch-Upstream-Status",
		"X-Dispatch-Upstream-Error-Code",
		"X-Dispatch-Upstream-Provider",
		"X-Dispatch-Upstream-Retryable",
	} {
		if v := rec.Header().Get(h); v != "" {
			t.Errorf("local error should not have %s = %q", h, v)
		}
	}
	snap := router.Stats.Snapshot()
	if snap.UpstreamErrorsTotal != 0 {
		t.Errorf("upstream_errors_total = %d, want 0 (invalid JSON is a local error)", snap.UpstreamErrorsTotal)
	}
}

func TestConnectionFailureCountsAsLocalError(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	upstream.Close()
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}

	for _, h := range []string{
		"X-Dispatch-Upstream-Status",
		"X-Dispatch-Upstream-Error-Code",
		"X-Dispatch-Upstream-Error-Type",
		"X-Dispatch-Upstream-Provider",
		"X-Dispatch-Upstream-Provider-Code",
		"X-Dispatch-Upstream-Is-BYOK",
		"X-Dispatch-Upstream-Retry-After",
		"X-Dispatch-Upstream-Retryable",
	} {
		if v := rec.Header().Get(h); v != "" {
			t.Errorf("connection failure should not set %s = %q", h, v)
		}
	}

	snap := router.Stats.Snapshot()
	if snap.UpstreamErrorsTotal != 0 {
		t.Errorf("upstream_errors_total = %d, want 0 (connection failure is a local proxy error)", snap.UpstreamErrorsTotal)
	}
	if snap.LocalProxyErrorsTotal != 1 {
		t.Errorf("local_proxy_errors_total = %d, want 1", snap.LocalProxyErrorsTotal)
	}
}

func TestStreamingConnectionFailureNoUpstreamHeaders(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	upstream.Close()
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	for _, h := range []string{
		"X-Dispatch-Upstream-Status",
		"X-Dispatch-Upstream-Error-Code",
		"X-Dispatch-Upstream-Error-Type",
		"X-Dispatch-Upstream-Provider",
		"X-Dispatch-Upstream-Retryable",
	} {
		if v := rec.Header().Get(h); v != "" {
			t.Errorf("streaming connection failure should not set %s = %q", h, v)
		}
	}

	snap := router.Stats.Snapshot()
	if snap.UpstreamErrorsTotal != 0 {
		t.Errorf("upstream_errors_total = %d, want 0", snap.UpstreamErrorsTotal)
	}
	if snap.LocalProxyErrorsTotal != 1 {
		t.Errorf("local_proxy_errors_total = %d, want 1", snap.LocalProxyErrorsTotal)
	}
}

func TestStatsDistinguishUpstreamVsLocalErrors(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.SetResponseHeaders = true

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-Test-Mode") == "connection-fail" {
			return
		}
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"code":429,"message":"rate limited","metadata":{"provider_name":"BadProvider"}}}`))
	}))
	t.Cleanup(upstream.Close)

	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	router := New(cfg, client)

	// Request 1: upstream 429
	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("req1 status = %d, want 429", rec.Code)
	}

	// Request 2: also upstream 429
	req2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.HandleChatCompletions(rec2, req2)

	snap := router.Stats.Snapshot()
	if snap.UpstreamErrorsTotal != 2 {
		t.Errorf("upstream_errors_total = %d, want 2", snap.UpstreamErrorsTotal)
	}
	if snap.LocalProxyErrorsTotal != 0 {
		t.Errorf("local_proxy_errors_total = %d, want 0", snap.LocalProxyErrorsTotal)
	}
	if snap.Upstream429Total != 2 {
		t.Errorf("upstream_429_total = %d, want 2", snap.Upstream429Total)
	}
	if snap.ByUpstreamProvider["BadProvider"] != 2 {
		t.Errorf("by_upstream_provider[BadProvider] = %d, want 2", snap.ByUpstreamProvider["BadProvider"])
	}
}

func TestLocalErrorDoesNotCreateRequestIndexEntry(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.RequestIndexEnabled = true

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cfg.OpenRouter.BaseURL = upstream.URL
	client := openrouter.NewClient(upstream.URL, "test-key", "", "dispatch")
	upstream.Close()
	router := New(cfg, client)

	body := `{"model":"dispatch/auto","messages":[{"role":"user","content":"hi"}],"stream":false}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.HandleChatCompletions(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
	requestID := rec.Header().Get("X-Dispatch-Request-Id")
	if requestID == "" {
		t.Fatal("request ID missing")
	}
	_, ok := router.RequestIndex.Lookup(requestID)
	if ok {
		t.Error("local proxy error should NOT create a request index entry")
	}
}

func TestUpstream429CreatesRequestIndexEntry(t *testing.T) {
	cfg, _ := config.DefaultConfig()
	cfg.Debug.RequestIndexEnabled = true

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"code":429,"message":"rate limited","metadata":{"provider_name":"BadProvider","error_type":"rate_limit_exceeded"}}}`))
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

	requestID := rec.Header().Get("X-Dispatch-Request-Id")
	meta, ok := router.RequestIndex.Lookup(requestID)
	if !ok {
		t.Fatal("upstream error should create request index entry")
	}
	if meta.Status != 429 {
		t.Errorf("status = %d, want 429", meta.Status)
	}
	if meta.UpstreamProvider != "BadProvider" {
		t.Errorf("upstream_provider = %s, want BadProvider", meta.UpstreamProvider)
	}
	if meta.UpstreamErrorType != "rate_limit_exceeded" {
		t.Errorf("upstream_error_type = %s, want rate_limit_exceeded", meta.UpstreamErrorType)
	}
}

func TestStatsEndpointShowsLocalProxyErrors(t *testing.T) {
	cfg, _ := config.DefaultConfig()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"code":429,"message":"rate limited","metadata":{"provider_name":"p"}}}`))
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

	statsReq := httptest.NewRequest(http.MethodGet, "/debug/stats", nil)
	statsRec := httptest.NewRecorder()
	router.HandleDebugStats(statsRec, statsReq)

	var snap StatsSnapshot
	json.NewDecoder(statsRec.Body).Decode(&snap)

	if snap.UpstreamErrorsTotal != 1 {
		t.Errorf("stats upstream_errors_total = %d, want 1", snap.UpstreamErrorsTotal)
	}
	if snap.LocalProxyErrorsTotal != 0 {
		t.Errorf("stats local_proxy_errors_total = %d, want 0", snap.LocalProxyErrorsTotal)
	}
	if _, ok := snap.ByUpstreamProvider["p"]; !ok {
		t.Error("stats should include by_upstream_provider")
	}
	if snap.ByStatus[429] != 1 {
		t.Errorf("stats by_status[429] = %d, want 1", snap.ByStatus[429])
	}
}
