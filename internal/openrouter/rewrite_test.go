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
