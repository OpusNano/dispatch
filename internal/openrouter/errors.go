package openrouter

import (
	"encoding/json"
	"net/http"
	"strconv"
)

const maxDiagnosticParseSize = 128 * 1024

type UpstreamErrorMeta struct {
	HTTPStatus      int    `json:"http_status"`
	ErrorCode       int    `json:"error_code,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
	ErrorType       string `json:"error_type,omitempty"`
	ProviderCode    string `json:"provider_code,omitempty"`
	ProviderName    string `json:"provider_name,omitempty"`
	IsBYOK          string `json:"is_byok,omitempty"`
	ModelSlug       string `json:"model_slug,omitempty"`
	RawTruncated    string `json:"raw_truncated,omitempty"`
	RetryAfter      string `json:"retry_after,omitempty"`
	Retryable       string `json:"retryable"`
	EmbeddedError   bool   `json:"embedded_error"`
	ErrorParsed     bool   `json:"error_parsed"`
	DiagnosticTrunc bool   `json:"diagnostic_truncated"`
}

func newUpstreamErrorMeta(status int) *UpstreamErrorMeta {
	em := &UpstreamErrorMeta{
		HTTPStatus: status,
		Retryable:  classifyRetryable(status, ""),
	}
	return em
}

func ExtractErrorMeta(resp *http.Response, body []byte) *UpstreamErrorMeta {
	if resp == nil {
		return nil
	}

	status := resp.StatusCode
	em := newUpstreamErrorMeta(status)

	if ra := resp.Header.Get("Retry-After"); ra != "" {
		em.RetryAfter = ra
	}

	parseLen := len(body)
	if parseLen > maxDiagnosticParseSize {
		parseLen = maxDiagnosticParseSize
		em.DiagnosticTrunc = true
	}

	parseSlice := body[:parseLen]

	if status >= 400 {
		parseErrorBody(em, parseSlice)
		return em
	}

	if parseEmbeddedError(em, parseSlice) {
		em.EmbeddedError = true
		em.ErrorParsed = true
		return em
	}

	return nil
}

func parseErrorBody(em *UpstreamErrorMeta, body []byte) {
	var resp struct {
		Error struct {
			Code     int             `json:"code"`
			Message  string          `json:"message"`
			Metadata json.RawMessage `json:"metadata"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &resp); err != nil || resp.Error.Code == 0 && resp.Error.Message == "" {
		return
	}

	em.ErrorParsed = true
	em.ErrorCode = resp.Error.Code
	em.ErrorMessage = resp.Error.Message

	if resp.Error.Code > 0 {
		em.Retryable = classifyRetryable(em.HTTPStatus, "")
	}

	if len(resp.Error.Metadata) > 0 {
		parseMetadata(em, resp.Error.Metadata)
	}

	if len(em.ErrorType) > 0 {
		em.Retryable = classifyRetryable(em.HTTPStatus, em.ErrorType)
	}
}

func parseEmbeddedError(em *UpstreamErrorMeta, body []byte) bool {
	var resp struct {
		Choices []struct {
			FinishReason string          `json:"finish_reason"`
			Error        json.RawMessage `json:"error"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return false
	}

	for _, choice := range resp.Choices {
		if choice.FinishReason == "error" && len(choice.Error) > 0 {
			parseInlineErrorBody(em, choice.Error)
			return true
		}
	}

	return false
}

func parseInlineErrorBody(em *UpstreamErrorMeta, body []byte) {
	var inner struct {
		Code     int             `json:"code"`
		Message  string          `json:"message"`
		Metadata json.RawMessage `json:"metadata"`
	}

	if err := json.Unmarshal(body, &inner); err != nil || inner.Code == 0 && inner.Message == "" {
		return
	}

	em.ErrorParsed = true
	em.ErrorCode = inner.Code
	em.ErrorMessage = inner.Message

	if inner.Code > 0 {
		em.Retryable = classifyRetryable(em.HTTPStatus, "")
	}

	if len(inner.Metadata) > 0 {
		parseMetadata(em, inner.Metadata)
	}

	if len(em.ErrorType) > 0 {
		em.Retryable = classifyRetryable(em.HTTPStatus, em.ErrorType)
	}
}

func parseMetadata(em *UpstreamErrorMeta, raw json.RawMessage) {
	var meta struct {
		ErrorType    string `json:"error_type"`
		ProviderCode string `json:"provider_code"`
		ProviderName string `json:"provider_name"`
		IsBYOK       any    `json:"is_byok"`
		ModelSlug    string `json:"model_slug"`
		Raw          string `json:"raw"`
	}

	if err := json.Unmarshal(raw, &meta); err != nil {
		return
	}

	em.ErrorType = meta.ErrorType
	em.ProviderCode = meta.ProviderCode
	em.ProviderName = meta.ProviderName
	em.ModelSlug = meta.ModelSlug

	switch v := meta.IsBYOK.(type) {
	case bool:
		em.IsBYOK = strconv.FormatBool(v)
	case string:
		em.IsBYOK = v
	}

	if len(meta.Raw) > 0 {
		if len(meta.Raw) > 300 {
			em.RawTruncated = meta.Raw[:300]
		} else {
			em.RawTruncated = meta.Raw
		}
	}
}

func classifyRetryable(httpStatus int, errorType string) string {
	switch httpStatus {
	case 429:
		return "true"
	case 408:
		return "true"
	case 502:
		return "true"
	case 503:
		return "true"
	case 504:
		return "true"
	case 400, 401, 402, 403, 404, 412, 413, 422:
		return "false"
	}

	switch errorType {
	case "rate_limit_exceeded", "provider_overloaded", "provider_unavailable", "timeout":
		return "true"
	case "invalid_request", "invalid_prompt", "authentication", "permission_denied",
		"payment_required", "not_found", "precondition_failed", "payload_too_large",
		"unprocessable", "content_policy_violation", "refusal":
		return "false"
	}

	return "unknown"
}

func (em *UpstreamErrorMeta) SetDiagnosticHeaders(w http.ResponseWriter) {
	if em == nil {
		return
	}

	setIf := func(key, val string) {
		if val != "" {
			w.Header().Set(key, val)
		}
	}

	setIf("X-Dispatch-Upstream-Status", strconv.Itoa(em.HTTPStatus))

	if em.ErrorCode > 0 {
		setIf("X-Dispatch-Upstream-Error-Code", strconv.Itoa(em.ErrorCode))
	}
	setIf("X-Dispatch-Upstream-Error-Type", em.ErrorType)
	setIf("X-Dispatch-Upstream-Provider-Code", em.ProviderCode)
	setIf("X-Dispatch-Upstream-Provider", em.ProviderName)
	setIf("X-Dispatch-Upstream-Is-BYOK", em.IsBYOK)
	setIf("X-Dispatch-Upstream-Retry-After", em.RetryAfter)
	setIf("X-Dispatch-Upstream-Retryable", em.Retryable)
}
