package openrouter

import (
	"encoding/json"
	"testing"

	"dispatch/internal/config"
)

func TestProviderMergeClientUnknownFieldsPreserved(t *testing.T) {
	clientJSON := `{"order":["openai"],"custom_field":"keep-me","quantity":3}`
	level := config.ProviderConfig{
		DataCollection: "deny",
	}
	body := `{"model":"x","messages":[],"provider":` + clientJSON + `}`
	result, err := RewriteRequest([]byte(body), "y", level)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)
	provider, _ := parsed["provider"].(map[string]interface{})
	if provider == nil {
		t.Fatal("provider missing")
	}
	if provider["custom_field"] != "keep-me" {
		t.Errorf("client custom_field lost: %v", provider["custom_field"])
	}
	if provider["quantity"] != float64(3) {
		t.Errorf("client quantity lost: %v", provider["quantity"])
	}
	if provider["data_collection"] != "deny" {
		t.Errorf("data_collection = %v, want deny", provider["data_collection"])
	}
	if provider["order"] == nil {
		t.Error("client order lost")
	}
}

func TestProviderMergeAllowFallbacksFalseStaysFalse(t *testing.T) {
	clientJSON := `{"allow_fallbacks":true}`
	level := config.ProviderConfig{
		AllowFallbacks: boolPtr(false),
	}
	body := `{"model":"x","messages":[],"provider":` + clientJSON + `}`
	result, err := RewriteRequest([]byte(body), "y", level)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)
	provider, _ := parsed["provider"].(map[string]interface{})
	if provider == nil {
		t.Fatal("provider missing")
	}
	if provider["allow_fallbacks"] != false {
		t.Errorf("allow_fallbacks = %v, want false", provider["allow_fallbacks"])
	}
}

func TestProviderMergeAllowFallbacksTrueStaysTrue(t *testing.T) {
	clientJSON := `{"allow_fallbacks":false}`
	level := config.ProviderConfig{
		AllowFallbacks: boolPtr(true),
	}
	body := `{"model":"x","messages":[],"provider":` + clientJSON + `}`
	result, err := RewriteRequest([]byte(body), "y", level)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)
	provider, _ := parsed["provider"].(map[string]interface{})
	if provider == nil {
		t.Fatal("provider missing")
	}
	if provider["allow_fallbacks"] != true {
		t.Errorf("allow_fallbacks = %v, want true", provider["allow_fallbacks"])
	}
}

func TestProviderMergeDataCollectionApplied(t *testing.T) {
	clientJSON := `{"data_collection":"allow"}`
	level := config.ProviderConfig{
		DataCollection: "deny",
	}
	body := `{"model":"x","messages":[],"provider":` + clientJSON + `}`
	result, err := RewriteRequest([]byte(body), "y", level)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)
	provider, _ := parsed["provider"].(map[string]interface{})
	if provider == nil {
		t.Fatal("provider missing")
	}
	if provider["data_collection"] != "deny" {
		t.Errorf("data_collection = %v, want deny (level overrides)", provider["data_collection"])
	}
}

func TestProviderMergeEmptyProviderNoObject(t *testing.T) {
	body := `{"model":"x","messages":[]}`
	result, err := RewriteRequest([]byte(body), "y", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)
	if parsed["provider"] != nil {
		t.Errorf("empty provider should not add provider object, got %v", parsed["provider"])
	}
}

func TestProviderMergeEmptyClientProviderPreserved(t *testing.T) {
	body := `{"model":"x","messages":[],"provider":{}}`
	result, err := RewriteRequest([]byte(body), "y", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)
	provider, _ := parsed["provider"].(map[string]interface{})
	if provider == nil {
		t.Error("empty client provider object should be preserved as empty object, not removed")
	}
}

func TestRewritePreservesAllUnknownFields(t *testing.T) {
	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"test"}],"temperature":0.7,"top_p":0.9,"max_tokens":4096,"stream":true,"stream_options":{"include_usage":true},"stop":["\n"],"n":1,"seed":42,"presence_penalty":0.5,"frequency_penalty":0.5,"logit_bias":{"123":1},"user":"user-123","custom_param":{"nested":{"deep":true}}}`)
	result, err := RewriteRequest(body, "test-model", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	required := []string{"temperature", "top_p", "max_tokens", "stream", "stream_options", "stop", "n", "seed", "presence_penalty", "frequency_penalty", "logit_bias", "user", "custom_param"}
	for _, key := range required {
		if parsed[key] == nil {
			t.Errorf("field %q lost in rewrite", key)
		}
	}
}

func TestRewriteInvalidJSON(t *testing.T) {
	_, err := RewriteRequest([]byte(`{invalid json`), "model", config.ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRewriteEmptyBody(t *testing.T) {
	_, err := RewriteRequest([]byte{}, "model", config.ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestRewriteNilBody(t *testing.T) {
	_, err := RewriteRequest(nil, "model", config.ProviderConfig{})
	if err == nil {
		t.Fatal("expected error for nil body")
	}
}

func TestRewritePreservesNumberPrecision(t *testing.T) {
	body := []byte(`{"model":"x","messages":[],"temperature":0.7}`)
	result, err := RewriteRequest(body, "y", config.ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	temp, ok := parsed["temperature"].(float64)
	if !ok {
		t.Errorf("temperature is not float64: %T", parsed["temperature"])
	}
	if temp != 0.7 {
		t.Errorf("temperature = %v, want 0.7", temp)
	}
}
