package openrouter

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"dispatch/internal/classifier"
)

type Client struct {
	BaseURL     string
	APIKey      string
	HTTPReferer string
	SiteTitle   string
	HTTPClient  *http.Client
}

const maxReasonsHeaderLen = 1024

func NewClient(baseURL, apiKey, httpReferer, siteTitle string) *Client {
	return &Client{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		HTTPReferer: httpReferer,
		SiteTitle:   siteTitle,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
				MaxIdleConns:    100,
				IdleConnTimeout: 90 * time.Second,
			},
			Timeout: 0,
		},
	}
}

func SetRoutingHeaders(w http.ResponseWriter, classification classifier.Classification) {
	w.Header().Set("X-Dispatch-Level", classification.Level)
	w.Header().Set("X-Dispatch-Model", classification.Model)
	w.Header().Set("X-Dispatch-Score-Total", fmt.Sprintf("%.1f", classification.Scores.Total))
	w.Header().Set("X-Dispatch-Score-Complexity", fmt.Sprintf("%.1f", classification.Scores.Complexity))
	w.Header().Set("X-Dispatch-Score-Risk", fmt.Sprintf("%.1f", classification.Scores.Risk))
	w.Header().Set("X-Dispatch-Score-Agent-Pressure", fmt.Sprintf("%.1f", classification.Scores.AgentPressure))

	reasons := truncateReasons(classification.Reasons)
	if reasons != "" {
		w.Header().Set("X-Dispatch-Reasons", reasons)
	}

	if classification.ForcedBy != "" {
		w.Header().Set("X-Dispatch-Forced-By", classification.ForcedBy)
	}
}

func truncateReasons(reasons []string) string {
	var buf strings.Builder
	for i, r := range reasons {
		part := r
		if i > 0 {
			if buf.Len()+len(part)+2 > maxReasonsHeaderLen {
				buf.WriteString(", ...")
				break
			}
			buf.WriteString(", ")
		}
		if buf.Len()+len(part) > maxReasonsHeaderLen {
			buf.WriteString("...")
			break
		}
		buf.WriteString(part)
	}
	return buf.String()
}

func (c *Client) ForwardStreaming(ctx context.Context, w http.ResponseWriter, body []byte) error {
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create upstream request: %w", err)
	}
	c.setAuthHeaders(upstreamReq)

	resp, err := c.HTTPClient.Do(upstreamReq)
	if err != nil {
		return fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	copyUpstreamHeaders(w, resp)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return nil
			}
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			slog.Debug("upstream read error", "error", err)
			return nil
		}
	}
}

func (c *Client) ForwardNonStream(ctx context.Context, w http.ResponseWriter, body []byte) error {
	upstreamReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create upstream request: %w", err)
	}
	c.setAuthHeaders(upstreamReq)

	resp, err := c.HTTPClient.Do(upstreamReq)
	if err != nil {
		return fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	copyUpstreamHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	return nil
}

func (c *Client) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if c.HTTPReferer != "" {
		req.Header.Set("HTTP-Referer", c.HTTPReferer)
	}
	if c.SiteTitle != "" {
		req.Header.Set("X-OpenRouter-Title", c.SiteTitle)
	}
}

func copyUpstreamHeaders(w http.ResponseWriter, upstream *http.Response) {
	skip := map[string]bool{
		"content-length":    true,
		"transfer-encoding": true,
		"connection":        true,
	}
	for key, values := range upstream.Header {
		if skip[strings.ToLower(key)] {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
}

func LogRouteMetrics(classification classifier.Classification, duration time.Duration, statusCode int, stream bool, bodySize int, requestID string) {
	if classification.ForcedBy != "" {
		slog.Info("route",
			"request_id", requestID,
			"level", classification.Level,
			"model", classification.Model,
			"forced_by", classification.ForcedBy,
			"score_total", fmt.Sprintf("%.1f", classification.Scores.Total),
			"score_complexity", fmt.Sprintf("%.1f", classification.Scores.Complexity),
			"score_risk", fmt.Sprintf("%.1f", classification.Scores.Risk),
			"score_agent_pressure", fmt.Sprintf("%.1f", classification.Scores.AgentPressure),
			"stream", stream,
			"status", statusCode,
			"duration_ms", duration.Milliseconds(),
			"body_bytes", bodySize,
			"reasons", classification.Reasons,
		)
		return
	}
	slog.Info("route",
		"request_id", requestID,
		"level", classification.Level,
		"model", classification.Model,
		"score_total", fmt.Sprintf("%.1f", classification.Scores.Total),
		"score_complexity", fmt.Sprintf("%.1f", classification.Scores.Complexity),
		"score_risk", fmt.Sprintf("%.1f", classification.Scores.Risk),
		"score_agent_pressure", fmt.Sprintf("%.1f", classification.Scores.AgentPressure),
		"stream", stream,
		"status", statusCode,
		"duration_ms", duration.Milliseconds(),
		"body_bytes", bodySize,
		"reasons", classification.Reasons,
	)
}
