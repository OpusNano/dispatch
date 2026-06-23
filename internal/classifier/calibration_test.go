package classifier

import (
	"testing"

	"dispatch/internal/config"
)

func TestCalibrationSet(t *testing.T) {
	cfg := loadConfig(t)

	tests := []struct {
		name       string
		text       string
		hasTools   bool
		hasRF      bool
		wantLevels []string
	}{
		{
			name:       "simple shell command question",
			text:       "how do I list files in a directory",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "simple TypeScript error explanation",
			text:       "what does 'cannot find name' mean in TypeScript",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "write a small test",
			text:       "write a unit test for this function",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "fix compile error",
			text:       "fix this compile error: undefined reference to foo",
			wantLevels: []string{"medium", "hard"},
		},
		{
			name:       "edit one file",
			text:       "edit the config to add a new field",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "multi-file refactor",
			text:       "refactor the architecture across multiple files with a complete redesign",
			wantLevels: []string{"hard"},
		},
		{
			name:       "architecture decision with tradeoffs",
			text:       "should we use a monorepo or polyrepo for this project? design the architecture and weigh the tradeoffs",
			wantLevels: []string{"hard"},
		},
		{
			name:       "suspicious auth bug — topic only, no escalation",
			text:       "there might be an authentication vulnerability in the login flow, possibly an XSS",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "production deploy failing — outage evidence",
			text:       "the production deploy is failing, customer-facing service is down with server log showing crash",
			wantLevels: []string{"hard", "critical"},
		},
		{
			name:       "database migration staging — no evidence",
			text:       "we need to run a database migration on the staging environment, alter table to add column",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "production database migration — topic only",
			text:       "we need a production database migration with rollback plan",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "secrets accidentally committed",
			text:       "the API key was accidentally committed to the public git repo, help rotate it",
			wantLevels: []string{"critical"},
		},
		{
			name:       "tool failed twice",
			text:       "the tool call failed twice, same error persists, exit code 1",
			wantLevels: []string{"hard"},
		},
		{
			name:       "patch did not apply",
			text:       "the patch did not apply cleanly, build failed with compile error",
			wantLevels: []string{"medium", "hard"},
		},
		{
			name:       "explain authentication in one sentence",
			text:       "explain authentication in one sentence",
			wantLevels: []string{"easy"},
		},
		{
			name:       "rename database variable",
			text:       "rename the database variable to dbConn",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "payment button CSS",
			text:       "payment button CSS styling",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "security section in README",
			text:       "add a security section in the README",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "toy local database migration",
			text:       "toy local database migration example",
			wantLevels: []string{"easy", "medium"},
		},
		{
			name:       "user says use critical model",
			text:       "dispatch/critical",
			wantLevels: []string{"easy", "medium", "hard", "critical"},
		},
	}

	validLevels := map[string]bool{"easy": true, "medium": true, "hard": true, "critical": true}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := Input{
				Messages:          []Message{{Role: "user", Content: tt.text}},
				HasTools:          tt.hasTools,
				HasResponseFormat: tt.hasRF,
			}
			result := ClassifySimple(input, cfg)
			if !validLevels[result.Level] {
				t.Errorf("invalid level %q", result.Level)
			}
			allowed := false
			for _, l := range tt.wantLevels {
				if result.Level == l {
					allowed = true
				}
			}
			if !allowed {
				t.Errorf("level = %s, want one of %v (scores: %+v, total=%.1f, reasons: %v)",
					result.Level, tt.wantLevels, result.Scores, result.Scores.Total, result.Reasons)
			}
		})
	}
}

func TestCalibrationForcedLevelModel(t *testing.T) {
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	input := Input{
		Messages: []Message{{Role: "user", Content: "use critical model"}},
	}
	result := ClassifySimple(input, cfg)
	if result.Level == "critical" {
		if result.Model != cfg.Levels["critical"].Model {
			t.Errorf("critical model = %s, want %s", result.Model, cfg.Levels["critical"].Model)
		}
	}
}
