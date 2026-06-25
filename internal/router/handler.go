package router

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"dispatch/internal/classifier"
	"dispatch/internal/config"
	"dispatch/internal/openrouter"
	"dispatch/internal/version"
)

type Router struct {
	cfg          atomic.Pointer[config.Config]
	Client       *openrouter.Client
	Stats        *Stats
	RequestIndex *requestIndex
}

func New(cfg *config.Config, client *openrouter.Client) *Router {
	rt := &Router{
		Client:       client,
		Stats:        NewStats(),
		RequestIndex: newRequestIndex(cfg.Debug.RequestIndexSize),
	}
	rt.cfg.Store(cfg)
	return rt
}

func (rt *Router) GetConfig() *config.Config {
	return rt.cfg.Load()
}

func (rt *Router) SwapConfig(newCfg *config.Config) {
	rt.cfg.Store(newCfg)
	rt.Stats.RecordReload(time.Now().Unix())
}

func (rt *Router) HandleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := rt.GetConfig()

	requestID := generateRequestID()
	w.Header().Set("X-Dispatch-Request-Id", requestID)

	startTime := time.Now()
	statusCode := http.StatusOK

	r.Body = http.MaxBytesReader(w, r.Body, cfg.Server.MaxBodySize)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	defer r.Body.Close()

	parsed, isStream, err := parseRequestForClassifier(rawBody)
	if err != nil {
		slog.Info("invalid request", "request_id", requestID, "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if cfg.Debug.TraceRequests {
		logRequestTrace(requestID, parsed, isStream, cfg)
	}

	if rt.Client.APIKey == "" {
		slog.Error("api key not configured", "request_id", requestID)
		http.Error(w, "OpenRouter API key not configured", http.StatusServiceUnavailable)
		return
	}

	level, forcedBy := rt.determineOverride(r, rawBody)

	var cls classifier.Classification
	var frame classifier.TaskFrame
	if forcedBy == "" {
		sessionID := r.Header.Get("X-Dispatch-Session-Id")
		taskID := r.Header.Get("X-Dispatch-Task-Id")
		profileName := ""
		if cfg.Intelligence != nil && cfg.Intelligence.RoutingProfiles.AllowHeaderOverride {
			profileName = r.Header.Get("X-Dispatch-Profile")
		}

		frame = classifier.ExtractTaskFrame(parsed.Messages, taskID, cfg)
		taskKey := frame.TaskKey

		frameInput := classifier.Input{
			Messages:          parsed.Messages[frame.TaskBoundaryIndex:],
			HasTools:          parsed.HasTools,
			HasResponseFormat: parsed.HasResponseFormat,
		}
		cls = classifier.Classify(frameInput, cfg, sessionID, taskKey, profileName)
		cls.Frame = &frame
		level = cls.Level
	}

	rm, ok := cfg.ResolveLevel(level)
	if !ok {
		http.Error(w, "unknown level", http.StatusInternalServerError)
		return
	}

	if forcedBy != "" {
		cls = classifier.Classification{
			Level:    level,
			Model:    rm.Model,
			ForcedBy: forcedBy,
		}
	}

	rewrittenBody, err := openrouter.RewriteRequest(rawBody, rm.Model, rm.Provider)
	if err != nil {
		slog.Error("rewrite failed", "error", err, "request_id", requestID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if cfg.Debug.SetResponseHeaders {
		openrouter.SetRoutingHeaders(w, cls)
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var upstreamMeta *openrouter.UpstreamErrorMeta
	if isStream {
		m, ferr := rt.Client.ForwardStreaming(ctx, w, rewrittenBody)
		if ferr != nil {
			rt.Stats.RecordLocalError()
			slog.Error("streaming forward failed", "error", ferr, "request_id", requestID)
			return
		}
		upstreamMeta = m
	} else {
		m, ferr := rt.Client.ForwardNonStream(ctx, w, rewrittenBody)
		if ferr != nil {
			rt.Stats.RecordLocalError()
			statusCode = http.StatusBadGateway
			slog.Error("non-stream forward failed", "error", ferr, "request_id", requestID)
			http.Error(w, "upstream error", statusCode)
			return
		}
		upstreamMeta = m
	}

	if upstreamMeta != nil {
		statusCode = upstreamMeta.HTTPStatus
	}

	routeDurationNs := time.Since(startTime).Nanoseconds()

	if cfg.Debug.LogDecisions {
		logDecision(requestID, cls, frame, isStream, statusCode, routeDurationNs, len(parsed.Messages), cfg)
	}
	if cfg.Debug.LogMetadata {
		openrouter.LogRouteMetrics(cls, time.Since(startTime), statusCode, isStream, len(rawBody), requestID)
	}

	sessionEscalated := false
	gateFired := false
	lengthCapped := false
	topicIgnored := false
	var gateName string

	if cls.Frame != nil {
		sessionEscalated = cls.Frame.SessionUsedPreviousState || cls.Frame.SessionUsedCurrentFrame
	}
	if len(cls.Analysis.CriticalGates) > 0 {
		gateFired = true
		gateName = cls.Analysis.CriticalGates[0]
	}
	if cls.Analysis.LengthPolicy == "capped_at_medium_without_evidence" {
		lengthCapped = true
	}
	if cls.Analysis.TopicEscalation == "ignored_by_policy" {
		topicIgnored = true
	}

	var errType, errProv, errProvCode string
	var embeddedErr bool
	if upstreamMeta != nil {
		errType = upstreamMeta.ErrorType
		errProv = upstreamMeta.ProviderName
		errProvCode = upstreamMeta.ProviderCode
		embeddedErr = upstreamMeta.EmbeddedError
	}
	rt.Stats.Record(level, rm.Model, statusCode, isStream, routeDurationNs,
		sessionEscalated, gateFired, frame.ContinuationDetected, lengthCapped, topicIgnored,
		errType, errProv, errProvCode, embeddedErr)

	if cfg.Debug.RequestIndexEnabled && cls.Frame != nil {
		latestUserHash := hashShort(extractLatestUserText(parsed.Messages, frame.LatestUserIndex))
		frameHash := hashShort(frame.FrameText)
		meta := &RequestMeta{
			RequestID:         requestID,
			Timestamp:         time.Now(),
			Level:             level,
			Model:             rm.Model,
			Status:            statusCode,
			LatestUserIndex:   frame.LatestUserIndex,
			TaskBoundaryIndex: frame.TaskBoundaryIndex,
			TaskKey:           frame.TaskKey,
			LatestUserHash:    latestUserHash,
			FrameHash:         frameHash,
			ReasonSummary:     truncateReasonsList(cls.Reasons, 10),
			CriticalGate:      gateName,
			SessionEscalated:  sessionEscalated,
		}
		if upstreamMeta != nil {
			meta.UpstreamErrorCode = upstreamMeta.ErrorCode
			meta.UpstreamErrorType = upstreamMeta.ErrorType
			meta.UpstreamProviderCode = upstreamMeta.ProviderCode
			meta.UpstreamProvider = upstreamMeta.ProviderName
			meta.UpstreamIsBYOK = upstreamMeta.IsBYOK
			meta.UpstreamRetryAfter = upstreamMeta.RetryAfter
			meta.UpstreamRetryable = upstreamMeta.Retryable
			meta.UpstreamRawTruncated = upstreamMeta.RawTruncated
			meta.EmbeddedError = upstreamMeta.EmbeddedError
			if len(upstreamMeta.ErrorMessage) > 300 {
				meta.UpstreamErrorMsg = upstreamMeta.ErrorMessage[:300]
			} else {
				meta.UpstreamErrorMsg = upstreamMeta.ErrorMessage
			}
		}
		rt.RequestIndex.Store(meta)
	}

	if upstreamMeta != nil && cfg.Debug.LogMetadata {
		openrouter.LogUpstreamError(requestID, upstreamMeta, level, rm.Model, time.Since(startTime))
	}
}

func (rt *Router) determineOverride(r *http.Request, rawBody []byte) (level string, forcedBy string) {
	if headerLevel := r.Header.Get("X-Dispatch-Level"); headerLevel != "" {
		headerLevel = strings.ToLower(strings.TrimSpace(headerLevel))
		if validLevel(headerLevel) {
			return headerLevel, "header:X-Dispatch-Level"
		}
	}

	model := extractModel(rawBody)
	if strings.HasPrefix(model, "dispatch/") {
		alias := strings.TrimPrefix(model, "dispatch/")
		if alias == "auto" {
			return "", ""
		}
		if validLevel(alias) {
			return alias, "model-alias:" + model
		}
	}

	return "", ""
}

func validLevel(level string) bool {
	switch level {
	case "easy", "medium", "hard", "critical":
		return true
	}
	return false
}

func extractModel(rawBody []byte) string {
	var body struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		return ""
	}
	return body.Model
}

func parseRequestForClassifier(rawBody []byte) (classifier.Input, bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &raw); err != nil {
		return classifier.Input{}, false, fmt.Errorf("invalid JSON: %w", err)
	}

	var messages []classifier.Message
	if msgRaw, ok := raw["messages"]; ok {
		var rawMsgs []struct {
			Role      string          `json:"role"`
			Content   json.RawMessage `json:"content"`
			ToolCalls json.RawMessage `json:"tool_calls"`
		}
		if err := json.Unmarshal(msgRaw, &rawMsgs); err == nil {
			for _, rm := range rawMsgs {
				text := extractTextFromContent(rm.Content)
				text += extractTextFromToolCalls(rm.ToolCalls)
				messages = append(messages, classifier.Message{
					Role:    rm.Role,
					Content: text,
				})
			}
		}
	}

	hasTools := openrouter.ParseToolPresence(raw)
	hasResponseFormat := openrouter.ParseResponseFormatPresence(raw)
	isStream := openrouter.ParseStreamFlag(raw)

	return classifier.Input{
		Messages:          messages,
		HasTools:          hasTools,
		HasResponseFormat: hasResponseFormat,
	}, isStream, nil
}

func (rt *Router) HandleDebugRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg := rt.GetConfig()

	requestID := generateRequestID()
	w.Header().Set("X-Dispatch-Request-Id", requestID)

	r.Body = http.MaxBytesReader(w, r.Body, cfg.Server.MaxBodySize)
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	defer r.Body.Close()

	parsed, _, err := parseRequestForClassifier(rawBody)
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	frame := classifier.ExtractTaskFrame(parsed.Messages, "", cfg)
	frameInput := classifier.Input{
		Messages:          parsed.Messages[frame.TaskBoundaryIndex:],
		HasTools:          parsed.HasTools,
		HasResponseFormat: parsed.HasResponseFormat,
	}
	cls := classifier.Classify(frameInput, cfg, "", "", "")
	cls.Frame = &frame

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"level":        cls.Level,
		"model":        cls.Model,
		"scores":       cls.Scores,
		"reasons":      cls.Reasons,
		"analysis":     cls.Analysis,
		"frame":        cls.Frame,
		"intelligence": cls.Intelligence,
		"request_id":   requestID,
	})
}

func (rt *Router) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (rt *Router) HandleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version":    version.Version,
		"commit":     version.Commit,
		"build_time": version.BuildTime,
	})
}

func (rt *Router) HandleDebugStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rt.Stats.Snapshot())
}

func (rt *Router) HandleDebugRequestLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := rt.GetConfig()
	if !cfg.Debug.RequestIndexEnabled {
		http.Error(w, "request index disabled", http.StatusNotFound)
		return
	}
	requestID := r.URL.Query().Get("id")
	if requestID == "" {
		http.Error(w, "missing id parameter", http.StatusBadRequest)
		return
	}
	meta, ok := rt.RequestIndex.Lookup(requestID)
	if !ok {
		http.Error(w, "request_id not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

func (rt *Router) HandleDebugFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := rt.GetConfig()
	if !cfg.Debug.FeedbackEnabled {
		http.Error(w, "feedback disabled", http.StatusNotFound)
		return
	}

	var body struct {
		RequestID     string `json:"request_id"`
		ExpectedLevel string `json:"expected_level"`
		Note          string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	validLevels := map[string]bool{"easy": true, "medium": true, "hard": true, "critical": true}
	if !validLevels[body.ExpectedLevel] {
		http.Error(w, "invalid expected_level (must be easy|medium|hard|critical)", http.StatusBadRequest)
		return
	}

	if len(body.Note) > 500 {
		body.Note = body.Note[:500]
	}

	entry := map[string]interface{}{
		"request_id":     body.RequestID,
		"expected_level": body.ExpectedLevel,
		"note":           body.Note,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}

	line, _ := json.Marshal(entry)
	line = append(line, '\n')

	f, err := os.OpenFile(cfg.Debug.FeedbackPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("feedback: cannot open file", "error", err)
		http.Error(w, "feedback file error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		slog.Error("feedback: write failed", "error", err)
		http.Error(w, "feedback write error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (rt *Router) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/chat/completions", rt.HandleChatCompletions)
	mux.HandleFunc("/debug/route", rt.HandleDebugRoute)
	mux.HandleFunc("/debug/stats", rt.HandleDebugStats)
	mux.HandleFunc("/debug/request", rt.HandleDebugRequestLookup)
	mux.HandleFunc("/debug/feedback", rt.HandleDebugFeedback)
	mux.HandleFunc("/health", rt.HandleHealth)
	mux.HandleFunc("/version", rt.HandleVersion)
}

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func extractTextFromContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" {
		return ""
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	var rawParts []json.RawMessage
	if err := json.Unmarshal(raw, &rawParts); err == nil {
		var buf strings.Builder
		for _, partRaw := range rawParts {
			var part map[string]json.RawMessage
			if err := json.Unmarshal(partRaw, &part); err != nil {
				continue
			}
			var typ string
			if typeRaw, ok := part["type"]; ok {
				json.Unmarshal(typeRaw, &typ)
			}
			if typ == "text" || typ == "input_text" {
				if textRaw, ok := part["text"]; ok {
					var text string
					json.Unmarshal(textRaw, &text)
					buf.WriteString(text)
				}
			}
		}
		return buf.String()
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		if textRaw, ok := obj["text"]; ok {
			var text string
			json.Unmarshal(textRaw, &text)
			return text
		}
	}

	return ""
}

func extractTextFromToolCalls(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var calls []struct {
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &calls); err != nil {
		return ""
	}
	var buf strings.Builder
	for _, call := range calls {
		buf.WriteString(call.Function.Name)
		buf.WriteString(" ")
		buf.WriteString(call.Function.Arguments)
	}
	return buf.String()
}

func logDecision(requestID string, cls classifier.Classification, frame classifier.TaskFrame, isStream bool, statusCode int, routeDurationNs int64, msgCount int, cfg *config.Config) {
	var gateName string
	if len(cls.Analysis.CriticalGates) > 0 {
		gateName = cls.Analysis.CriticalGates[0]
	}
	lengthCapped := cls.Analysis.LengthPolicy == "capped_at_medium_without_evidence"
	topicIgnored := cls.Analysis.TopicEscalation == "ignored_by_policy"

	includedMsgs := msgCount - frame.TaskBoundaryIndex
	excludedMsgs := frame.TaskBoundaryIndex

	slog.Info("decision",
		"request_id", requestID,
		"level", cls.Level,
		"model", cls.Model,
		"route_duration_ms", routeDurationNs/1e6,
		"stream", isStream,
		"status", statusCode,
		"frame_latest_user_index", frame.LatestUserIndex,
		"frame_task_boundary_index", frame.TaskBoundaryIndex,
		"included_messages", includedMsgs,
		"excluded_prior_messages", excludedMsgs,
		"continuation_detected", frame.ContinuationDetected,
		"session_escalated", frame.SessionUsedPreviousState || frame.SessionUsedCurrentFrame,
		"critical_gate", gateName,
		"length_capped", lengthCapped,
		"topic_ignored", topicIgnored,
	)
}

func logRequestTrace(requestID string, input classifier.Input, isStream bool, cfg *config.Config) {
	roles := make([]string, len(input.Messages))
	toolMsgCount := 0
	assistantToolCallCount := 0
	for i, msg := range input.Messages {
		roles[i] = msg.Role
		if msg.Role == "tool" {
			toolMsgCount++
		}
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			assistantToolCallCount++
		}
	}

	contentTypes := make([]string, len(input.Messages))
	for i, msg := range input.Messages {
		if msg.Content != "" {
			contentTypes[i] = "text"
		} else if len(msg.ToolCalls) > 0 {
			contentTypes[i] = "tool_calls"
		} else {
			contentTypes[i] = "empty"
		}
	}

	slog.Info("trace",
		"request_id", requestID,
		"message_count", len(input.Messages),
		"roles", roles,
		"content_types", contentTypes,
		"tool_message_count", toolMsgCount,
		"assistant_tool_call_count", assistantToolCallCount,
		"has_tools", input.HasTools,
		"has_response_format", input.HasResponseFormat,
		"stream", isStream,
	)
}

func extractLatestUserText(messages []classifier.Message, idx int) string {
	if idx >= 0 && idx < len(messages) {
		return messages[idx].Content
	}
	return ""
}

func hashShort(text string) string {
	if text == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:8])
}

func truncateReasonsList(reasons []string, max int) []string {
	if len(reasons) <= max {
		return reasons
	}
	result := make([]string, max)
	copy(result, reasons[:max])
	return result
}
