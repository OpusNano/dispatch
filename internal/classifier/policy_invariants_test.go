package classifier

import (
	"testing"
)

func TestCriticalGatesNotDowngradable(t *testing.T) {
	cfg := loadCfg(t)

	tests := []struct {
		name string
		text string
	}{
		{"explain destructive production action", "explain the DROP TABLE production data"},
		{"summarize secret leak", "summarize this API key accidentally committed to the public git repo"},
		{"no edits explain auth bypass with impact", "no edits, just explain the auth bypass in production that lets users access other accounts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifySimple(makeInput(tt.text, false, false), cfg)
			if result.Level != "critical" {
				t.Errorf("level = %s, want critical (gates: %v, analysis: %+v)",
					result.Level, result.Analysis.CriticalGates, result.Analysis)
			}
			if len(result.Analysis.CriticalGates) == 0 {
				t.Error("expected at least one critical gate to fire")
			}
		})
	}
}

func TestHardFloors(t *testing.T) {
	cfg := loadCfg(t)

	tests := []struct {
		name     string
		text     string
		hasTools bool
		hasRF    bool
	}{
		{"debug with stack trace", "debug suspicious auth bug, tests failing with stack trace", false, false},
		{"investigate migration failure", "investigate database migration failure with stack trace and compile error", false, false},
		{"strict JSON schema with tool calls", "return the data as strict json", true, true},
		{"compile error after refactoring", "compile error after refactoring across multiple files", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifySimple(makeInput(tt.text, tt.hasTools, tt.hasRF), cfg)
			if result.Level != "hard" {
				t.Errorf("level = %s, want hard (floors: %v, analysis: %+v)",
					result.Level, result.Analysis.Floors, result.Analysis)
			}
		})
	}
}

func TestSafeDowngrades(t *testing.T) {
	cfg := loadCfg(t)

	t.Run("explain authentication in one sentence", func(t *testing.T) {
		result := ClassifySimple(makeInput("explain authentication in one sentence", false, false), cfg)
		if result.Level != "easy" {
			t.Errorf("level = %s, want easy", result.Level)
		}
	})

	t.Run("summarize security section in README", func(t *testing.T) {
		result := ClassifySimple(makeInput("summarize the security section in the README", false, false), cfg)
		if result.Level == "hard" || result.Level == "critical" {
			t.Errorf("level = %s, should not be hard/critical", result.Level)
		}
	})

	t.Run("toy local database migration for demo app", func(t *testing.T) {
		result := ClassifySimple(makeInput("toy local database migration for demo app", false, false), cfg)
		if result.Level == "hard" || result.Level == "critical" {
			t.Errorf("level = %s, should not be hard/critical (toy/local)", result.Level)
		}
	})

	t.Run("payment button CSS color change", func(t *testing.T) {
		result := ClassifySimple(makeInput("payment button CSS color change", false, false), cfg)
		if result.Level == "hard" || result.Level == "critical" {
			t.Errorf("level = %s, should not be hard/critical", result.Level)
		}
	})
}

func TestFloorNotBypassedByWeakDowngrades(t *testing.T) {
	cfg := loadCfg(t)

	t.Run("compile error refactor persists hard", func(t *testing.T) {
		result := ClassifySimple(makeInput("briefly fix this compile error after refactoring multiple files", false, false), cfg)
		if result.Level != "hard" {
			t.Errorf("level = %s, want hard (floors: %v)", result.Level, result.Analysis.Floors)
		}
	})

	t.Run("destructive action gate wins", func(t *testing.T) {
		result := ClassifySimple(makeInput("quickly DROP TABLE production data", false, false), cfg)
		if result.Level != "critical" {
			t.Errorf("level = %s, want critical (gates: %v)", result.Level, result.Analysis.CriticalGates)
		}
	})

	t.Run("secret leak gate wins", func(t *testing.T) {
		result := ClassifySimple(makeInput("small change: API key committed to public git repo, rotate it", false, false), cfg)
		if result.Level != "critical" {
			t.Errorf("level = %s, want critical (gates: %v)", result.Level, result.Analysis.CriticalGates)
		}
	})
}

func TestPolicyInvariants(t *testing.T) {
	cfg := loadCfg(t)

	t.Run("critical gate always produces critical regardless of score", func(t *testing.T) {
		text := "DROP TABLE production data"
		facts := ExtractFacts(text, false, false)
		scoreLevel := "easy"
		level, analysis, _ := EvaluatePolicy(facts, scoreLevel, text)
		if level != "critical" {
			t.Errorf("gate should force critical even with easy score, got %s (gates: %v)", level, analysis.CriticalGates)
		}
		if len(analysis.CriticalGates) == 0 {
			t.Error("expected critical gate to fire")
		}
	})

	t.Run("score cannot lower below floor", func(t *testing.T) {
		text := "compile error after refactoring multiple files"
		facts := ExtractFacts(text, false, false)
		scoreLevel := "easy"
		level, _, _ := EvaluatePolicy(facts, scoreLevel, text)
		if level != "hard" {
			t.Errorf("floor should prevent easy, got %s, want hard", level)
		}
	})

	t.Run("score can raise above floor", func(t *testing.T) {
		text := "hello world"
		facts := ExtractFacts(text, false, false)
		scoreLevel := "hard"
		level, _, _ := EvaluatePolicy(facts, scoreLevel, text)
		if level != "hard" {
			t.Errorf("score above floor should be preserved, got %s, want hard", level)
		}
	})

	t.Run("explain read-only removes floor for safe queries", func(t *testing.T) {
		text := "explain this compile error"
		facts := ExtractFacts(text, false, false)
		scoreLevel := "easy"
		level, _, _ := EvaluatePolicy(facts, scoreLevel, text)
		if level != "easy" {
			t.Errorf("explain/read-only should downgrade to easy, got %s", level)
		}
	})

	t.Run("toy local reduces hard floor to medium not easy", func(t *testing.T) {
		text := "toy refactor the code across multiple files in local environment"
		facts := ExtractFacts(text, false, false)
		scoreLevel := "easy"
		level, _, _ := EvaluatePolicy(facts, scoreLevel, text)
		if level != "medium" {
			t.Errorf("toy/local should reduce hard to medium, got %s", level)
		}
	})

	t.Run("deterministic same input same output", func(t *testing.T) {
		text := "debug auth bug in production with compile error and stack trace"
		r1 := ClassifySimple(makeInput(text, false, false), cfg)
		r2 := ClassifySimple(makeInput(text, false, false), cfg)
		if r1.Level != r2.Level {
			t.Errorf("non-deterministic: r1=%s r2=%s", r1.Level, r2.Level)
		}
		if len(r1.Reasons) != len(r2.Reasons) {
			t.Errorf("non-deterministic reasons: r1=%d r2=%d", len(r1.Reasons), len(r2.Reasons))
		}
	})
}
