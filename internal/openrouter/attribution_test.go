package openrouter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetAuthHeadersSendsAttributionWhenConfigured(t *testing.T) {
	var capturedHeaders http.Header

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-key", "https://github.com/OpusNano/dispatch", "Dispatch")
	_, err := client.ForwardNonStream(context.Background(), httptest.NewRecorder(), []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("ForwardNonStream: %v", err)
	}

	if got := capturedHeaders.Get("HTTP-Referer"); got != "https://github.com/OpusNano/dispatch" {
		t.Errorf("HTTP-Referer = %q, want %q", got, "https://github.com/OpusNano/dispatch")
	}
	if got := capturedHeaders.Get("X-OpenRouter-Title"); got != "Dispatch" {
		t.Errorf("X-OpenRouter-Title = %q, want %q", got, "Dispatch")
	}
	if got := capturedHeaders.Get("Authorization"); got != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer test-key")
	}
	if got := capturedHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
}

func TestSetAuthHeadersOmitsAttributionWhenEmpty(t *testing.T) {
	var capturedHeaders http.Header

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-key", "", "")
	_, err := client.ForwardNonStream(context.Background(), httptest.NewRecorder(), []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("ForwardNonStream: %v", err)
	}

	if vals := capturedHeaders.Values("HTTP-Referer"); len(vals) != 0 {
		t.Errorf("HTTP-Referer should not be sent when empty, got %v", vals)
	}
	if vals := capturedHeaders.Values("X-OpenRouter-Title"); len(vals) != 0 {
		t.Errorf("X-OpenRouter-Title should not be sent when empty, got %v", vals)
	}
}

func TestSetAuthHeadersPartialAttribution(t *testing.T) {
	var capturedHeaders http.Header

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-key", "https://example.com", "")
	_, err := client.ForwardNonStream(context.Background(), httptest.NewRecorder(), []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		t.Fatalf("ForwardNonStream: %v", err)
	}

	if got := capturedHeaders.Get("HTTP-Referer"); got != "https://example.com" {
		t.Errorf("HTTP-Referer = %q, want %q", got, "https://example.com")
	}
	if vals := capturedHeaders.Values("X-OpenRouter-Title"); len(vals) != 0 {
		t.Errorf("X-OpenRouter-Title should not be sent when empty, got %v", vals)
	}
}

func TestSetAuthHeadersStreamingSendsAttribution(t *testing.T) {
	var capturedHeaders http.Header

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n")
	}))
	defer ts.Close()

	client := NewClient(ts.URL, "test-key", "https://github.com/OpusNano/dispatch", "Dispatch")
	_, err := client.ForwardStreaming(context.Background(), httptest.NewRecorder(), []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	if err != nil {
		t.Fatalf("ForwardStreaming: %v", err)
	}

	if got := capturedHeaders.Get("HTTP-Referer"); got != "https://github.com/OpusNano/dispatch" {
		t.Errorf("HTTP-Referer (streaming) = %q, want %q", got, "https://github.com/OpusNano/dispatch")
	}
	if got := capturedHeaders.Get("X-OpenRouter-Title"); got != "Dispatch" {
		t.Errorf("X-OpenRouter-Title (streaming) = %q, want %q", got, "Dispatch")
	}
}
