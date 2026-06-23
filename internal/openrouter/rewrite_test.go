package openrouter

import (
	"encoding/json"
	"testing"

	"dispatch/internal/config"
)

func TestRewriteModel(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	result, err := RewriteRequest(body, "deepseek/deepseek-v4-flash", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["model"] != "deepseek/deepseek-v4-flash" {
		t.Errorf("model = %v, want deepseek/deepseek-v4-flash", parsed["model"])
	}
	if parsed["stream"] != true {
		t.Error("stream flag lost")
	}
}

func TestUnknownFieldsPreserved(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"test"}],"custom_field":"value","nested":{"a":1}}`)
	result, err := RewriteRequest(body, "test-model", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}

	if parsed["custom_field"] != "value" {
		t.Error("custom_field lost")
	}
}

func TestProviderMerge(t *testing.T) {
	tests := []struct {
		name   string
		client string
		level  config.ProviderConfig
		check  func(t *testing.T, provider map[string]interface{})
	}{
		{
			name:   "level sets data_collection",
			client: `{"order":["openai"]}`,
			level: config.ProviderConfig{
				DataCollection: "deny",
			},
			check: func(t *testing.T, p map[string]interface{}) {
				if p["data_collection"] != "deny" {
					t.Errorf("data_collection = %v, want deny", p["data_collection"])
				}
				if p["order"] == nil {
					t.Error("client order lost")
				}
			},
		},
		{
			name:   "level overrides order",
			client: `{"order":["openai"]}`,
			level: config.ProviderConfig{
				Order: []string{"together", "anthropic"},
			},
			check: func(t *testing.T, p map[string]interface{}) {
				order := p["order"]
				if order == nil {
					t.Fatal("order missing")
				}
				orders, ok := order.([]interface{})
				if !ok || len(orders) != 2 {
					t.Errorf("order = %v, want [together anthropic]", order)
				}
			},
		},
		{
			name:   "level overrides allow_fallbacks",
			client: `{"allow_fallbacks":true}`,
			level: config.ProviderConfig{
				AllowFallbacks: boolPtr(false),
			},
			check: func(t *testing.T, p map[string]interface{}) {
				if p["allow_fallbacks"] != false {
					t.Errorf("allow_fallbacks = %v, want false", p["allow_fallbacks"])
				}
			},
		},
		{
			name:   "no provider in request, level sets provider",
			client: `{}`,
			level: config.ProviderConfig{
				DataCollection: "deny",
				AllowFallbacks: boolPtr(false),
				Order:          []string{"anthropic"},
			},
			check: func(t *testing.T, p map[string]interface{}) {
				if p["data_collection"] != "deny" {
					t.Error("data_collection missing")
				}
				if p["allow_fallbacks"] != false {
					t.Error("allow_fallbacks missing")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"model":"x","messages":[],"provider":` + tt.client + `}`
			result, err := RewriteRequest([]byte(body), "y", tt.level)
			if err != nil {
				t.Fatal(err)
			}
			var parsed map[string]interface{}
			if err := json.Unmarshal(result, &parsed); err != nil {
				t.Fatal(err)
			}
			provider, _ := parsed["provider"].(map[string]interface{})
			if provider == nil {
				t.Fatal("provider missing in output")
			}
			tt.check(t, provider)
		})
	}
}

func TestParseToolPresence(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{`{"model":"x","messages":[]}`, false},
		{`{"model":"x","messages":[],"tools":[{"type":"function"}]}`, true},
		{`{"model":"x","messages":[],"tool_choice":"auto"}`, true},
	}

	for _, tt := range tests {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tt.body), &raw); err != nil {
			t.Fatal(err)
		}
		if got := ParseToolPresence(raw); got != tt.want {
			t.Errorf("ParseToolPresence(%s) = %v, want %v", tt.body, got, tt.want)
		}
	}
}

func TestParseResponseFormatPresence(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{`{"model":"x","messages":[]}`, false},
		{`{"model":"x","messages":[],"response_format":{"type":"json_object"}}`, true},
	}

	for _, tt := range tests {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tt.body), &raw); err != nil {
			t.Fatal(err)
		}
		if got := ParseResponseFormatPresence(raw); got != tt.want {
			t.Errorf("ParseResponseFormatPresence(%s) = %v, want %v", tt.body, got, tt.want)
		}
	}
}

func TestParseStreamFlag(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{
		{`{"model":"x","messages":[]}`, false},
		{`{"model":"x","messages":[],"stream":true}`, true},
		{`{"model":"x","messages":[],"stream":false}`, false},
	}

	for _, tt := range tests {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tt.body), &raw); err != nil {
			t.Fatal(err)
		}
		if got := ParseStreamFlag(raw); got != tt.want {
			t.Errorf("ParseStreamFlag(%s) = %v, want %v", tt.body, got, tt.want)
		}
	}
}

func TestMessagesPreservedByteEquivalent(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"system","content":"you are helpful"},{"role":"user","content":"hello"},{"role":"assistant","content":"hi there","tool_calls":[{"id":"1","function":{"name":"read","arguments":"{\"path\":\"x\"}"}}]},{"role":"tool","tool_call_id":"1","content":"file content here"}]}`)
	result, err := RewriteRequest(body, "deepseek/deepseek-v4-flash", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}

	var original, rewritten map[string]interface{}
	json.Unmarshal(body, &original)
	json.Unmarshal(result, &rewritten)

	origMsgs, _ := json.Marshal(original["messages"])
	rewrittenMsgs, _ := json.Marshal(rewritten["messages"])
	if string(origMsgs) != string(rewrittenMsgs) {
		t.Error("messages content modified — must be byte-equivalent")
	}
}

func TestToolsPreserved(t *testing.T) {
	body := []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function","function":{"name":"get_weather","description":"Get weather","parameters":{"type":"object"}}}],"tool_choice":"auto"}`)
	result, err := RewriteRequest(body, "y", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]interface{}
	json.Unmarshal(result, &p)
	if p["tools"] == nil {
		t.Error("tools field lost")
	}
	if p["tool_choice"] != "auto" {
		t.Error("tool_choice lost")
	}
}

func TestResponseFormatPreserved(t *testing.T) {
	body := []byte(`{"model":"x","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"json_object"}}`)
	result, err := RewriteRequest(body, "y", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]interface{}
	json.Unmarshal(result, &p)
	rf, ok := p["response_format"].(map[string]interface{})
	if !ok || rf["type"] != "json_object" {
		t.Error("response_format lost or modified")
	}
}

func TestNoDebugTextInForwardedMessages(t *testing.T) {
	tests := []string{
		"request_id",
		"X-Dispatch",
		"dispatch/",
		"/debug",
		"route_level",
	}
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"write a function to sort an array"}]}`)
	result, err := RewriteRequest(body, "test-model", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	resultStr := string(result)
	for _, banned := range tests {
		if containsCaseInsensitive(resultStr, banned) {
			t.Errorf("forwarded body contains banned text: %q", banned)
		}
	}
}

func TestOnlyModelAndProviderChanged(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"test"}],"temperature":0.7,"max_tokens":100,"top_p":0.9}`)
	result, err := RewriteRequest(body, "target-model", config.ProviderConfig{DataCollection: "deny"})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]interface{}
	json.Unmarshal(result, &p)
	if p["temperature"] != 0.7 {
		t.Error("temperature modified")
	}
	if p["max_tokens"] != float64(100) {
		t.Error("max_tokens modified")
	}
	if p["top_p"] != 0.9 {
		t.Error("top_p modified")
	}
	if p["model"] != "target-model" {
		t.Error("model not changed")
	}
}

func TestContentArrayMessagesPreserved(t *testing.T) {
	body := []byte(`{"model":"x","messages":[{"role":"user","content":[{"type":"text","text":"write a function"},{"type":"image_url","image_url":{"url":"http://example.com/image.png"}}]}]}`)
	result, err := RewriteRequest(body, "y", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var original, rewritten map[string]interface{}
	json.Unmarshal(body, &original)
	json.Unmarshal(result, &rewritten)
	origMsgs, _ := json.Marshal(original["messages"])
	rewrittenMsgs, _ := json.Marshal(rewritten["messages"])
	if string(origMsgs) != string(rewrittenMsgs) {
		t.Error("content array messages modified")
	}
}

func containsCaseInsensitive(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && containsFold(s, substr)
}

func containsFold(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func TestTruncateReasons(t *testing.T) {
	short := []string{"reason a", "reason b"}
	if got := truncateReasons(short); got != "reason a, reason b" {
		t.Errorf("short = %q", got)
	}

	var long []string
	for i := 0; i < 100; i++ {
		long = append(long, "this is a fairly long reason text that explains a classification decision")
	}
	result := truncateReasons(long)
	if len(result) > maxReasonsHeaderLen+10 {
		t.Errorf("truncated too long: %d bytes", len(result))
	}
}

func boolPtr(b bool) *bool { return &b }
