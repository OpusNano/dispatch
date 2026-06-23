package classifier

import (
	"strings"
	"testing"

	"dispatch/internal/config"
)

func BenchmarkClassifySmallPrompt(b *testing.B) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		b.Fatal(err)
	}
	input := Input{
		Messages: []Message{{Role: "user", Content: "hello world"}},
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ClassifySimple(input, cfg)
	}
}

func BenchmarkClassifyLargeOpenCodePrompt(b *testing.B) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		b.Fatal(err)
	}
	content := "I need to refactor the authentication module across multiple files. " +
		"First, fix the compile error in auth.go: undefined reference to ValidateToken. " +
		"Then run the tests - 3 tests are failing with the same error. " +
		"The stack trace shows a panic at auth.go:42. " +
		"This is a production system with customer-facing traffic. " +
		"There's a potential security vulnerability with the OAuth flow. " +
		"We also need a database migration to add a new column to the users table. " +
		"Make sure to handle the rollback plan. " +
		"The tool call failed with exit code 1. " +
		"Step by step: first fix the compile error, then add tests, finally deploy. " +
		"Here's the code:\n```\nfunc ValidateToken(token string) error {\n\treturn nil\n}\n```\n" +
		strings.Repeat("Additional context for the prompt. ", 50)
	input := Input{
		Messages: []Message{
			{Role: "system", Content: "You are a coding assistant."},
			{Role: "user", Content: content},
		},
		HasTools:          true,
		HasResponseFormat: true,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ClassifySimple(input, cfg)
	}
}
