package openrouter

import (
	"bytes"
	"encoding/json"
	"fmt"

	"dispatch/internal/config"
)

func RewriteRequest(body []byte, model string, provider config.ProviderConfig) ([]byte, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("empty request body")
	}

	raw := make(map[string]json.RawMessage)
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse request: %w", err)
	}

	m, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("marshal model: %w", err)
	}
	raw["model"] = m

	raw["provider"] = mergeProvider(raw["provider"], provider)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(raw); err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

func mergeProvider(clientRaw json.RawMessage, level config.ProviderConfig) json.RawMessage {
	var client map[string]json.RawMessage
	if len(clientRaw) > 0 {
		json.Unmarshal(clientRaw, &client)
	}
	if client == nil {
		client = make(map[string]json.RawMessage)
	}

	levelHasFields := len(level.Order) > 0 ||
		len(level.Only) > 0 ||
		len(level.Ignore) > 0 ||
		level.DataCollection != "" ||
		level.AllowFallbacks != nil

	if !levelHasFields {
		if len(clientRaw) > 0 {
			reencoded, _ := json.Marshal(client)
			return reencoded
		}
		return nil
	}

	if len(level.Order) > 0 {
		v, _ := json.Marshal(level.Order)
		client["order"] = v
	}

	if level.AllowFallbacks != nil {
		v, _ := json.Marshal(*level.AllowFallbacks)
		client["allow_fallbacks"] = v
	}

	if level.DataCollection != "" {
		v, _ := json.Marshal(level.DataCollection)
		client["data_collection"] = v
	}

	if len(level.Only) > 0 {
		v, _ := json.Marshal(level.Only)
		client["only"] = v
	}
	if len(level.Ignore) > 0 {
		v, _ := json.Marshal(level.Ignore)
		client["ignore"] = v
	}

	result, _ := json.Marshal(client)
	return result
}

func ParseToolPresence(raw map[string]json.RawMessage) bool {
	if _, ok := raw["tools"]; ok {
		return true
	}
	if _, ok := raw["tool_choice"]; ok {
		return true
	}
	return false
}

func ParseResponseFormatPresence(raw map[string]json.RawMessage) bool {
	_, ok := raw["response_format"]
	return ok
}

func ParseStreamFlag(raw map[string]json.RawMessage) bool {
	var stream bool
	if v, ok := raw["stream"]; ok {
		json.Unmarshal(v, &stream)
	}
	return stream
}
