package classifier

import (
	"testing"

	"dispatch/internal/config"
)

func loadCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestCriticalGateSecretLeak(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("the API key was accidentally committed to the public git repo, help rotate it", false, false), cfg)
	if result.Level != "critical" {
		t.Errorf("secret leak should be critical, got %s (gates: %v)", result.Level, result.Analysis.CriticalGates)
	}
	if len(result.Analysis.CriticalGates) == 0 {
		t.Error("expected at least one critical gate")
	}
}

func TestCriticalGatePaymentFailure(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("the payment refund is failing for stripe credit card transactions and customers are affected", false, false), cfg)
	if result.Level != "critical" {
		t.Errorf("payment transaction impact should be critical, got %s (gates: %v)", result.Level, result.Analysis.CriticalGates)
	}
}

func TestProductionDBMigrationTopicOnlyNotCritical(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("production database migration rollback", false, false), cfg)
	if result.Level == "critical" || result.Level == "hard" {
		t.Errorf("production DB migration with no evidence should NOT be hard/critical, got %s", result.Level)
	}
}

func TestAuthBypassWithoutImpactNotCritical(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("auth bypass in production", false, false), cfg)
	if result.Level == "critical" {
		t.Errorf("auth bypass without access impact should NOT be critical, got %s", result.Level)
	}
}

func TestAuthBypassWithAccessImpactCritical(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("auth bypass in production lets users access other customer accounts", false, false), cfg)
	if result.Level != "critical" {
		t.Errorf("auth bypass with access impact should be critical, got %s (gates: %v)", result.Level, result.Analysis.CriticalGates)
	}
}

func TestIrreversibleActionCritical(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("irreversible data loss from truncate in production database", false, false), cfg)
	if result.Level != "critical" {
		t.Errorf("irreversible action with production context should be critical, got %s (gates: %v)", result.Level, result.Analysis.CriticalGates)
	}
}

func TestDataLossWithoutContextNotCritical(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("this operation will cause data loss if we proceed", false, false), cfg)
	if result.Level == "critical" {
		t.Errorf("data loss without production/database/customer context should NOT be critical, got %s", result.Level)
	}
}

func TestTopicOnlyNotHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("suspicious auth bug", false, false), cfg)
	if result.Level == "hard" || result.Level == "critical" {
		t.Errorf("topic-only auth bug should NOT be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestDatabaseMigrationWithoutEvidenceNotHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("we need to run a database migration on the staging environment, alter table to add column", false, false), cfg)
	if result.Level == "hard" {
		t.Errorf("database migration without failure evidence should NOT be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestDatabaseRollbackWithoutEvidenceNotHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("rollback the database changes", false, false), cfg)
	if result.Level == "hard" {
		t.Errorf("database rollback without failure evidence should NOT be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestSecurityReviewWithoutEvidenceNotHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("security review of the authentication module", false, false), cfg)
	if result.Level == "hard" {
		t.Errorf("security review without failure evidence should NOT be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestArchitectureDecisionWithTradeoffsHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("design the system architecture for the new API, weigh tradeoffs", false, false), cfg)
	if result.Level != "hard" {
		t.Errorf("architecture decision with tradeoff analysis should be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestArchitectureDecisionWithoutEvidenceMedium(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("design the system architecture for the new API", false, false), cfg)
	if result.Level == "hard" {
		t.Errorf("architecture decision without complexity evidence should NOT be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestMultiFileRefactorFloorHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("refactor the architecture across multiple files", false, false), cfg)
	if result.Level != "hard" {
		t.Errorf("multi-file refactor should be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestCompileErrorMedium(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("fix this compile error: undefined reference to foo", false, false), cfg)
	if result.Level != "medium" && result.Level != "hard" {
		t.Errorf("compile error should be at least medium, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestStrictJsonWithToolsHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(Input{
		Messages:          []Message{{Role: "user", Content: "return the data as json"}},
		HasTools:          true,
		HasResponseFormat: true,
	}, cfg)
	if result.Level != "hard" {
		t.Errorf("strict json with tools should be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestDowngradeExplainRemovesFloor(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("explain authentication in one sentence", false, false), cfg)
	if result.Level != "easy" {
		t.Errorf("explain auth in one sentence should be easy, got %s", result.Level)
	}
}

func TestDowngradeToyReducesFloor(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("a toy local database migration example", false, false), cfg)
	if result.Level == "hard" || result.Level == "critical" {
		t.Errorf("toy local migration should not be hard/critical, got %s", result.Level)
	}
}

func TestPaymentButtonCSSNotCriticalNotHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("payment button CSS", false, false), cfg)
	if result.Level == "critical" {
		t.Errorf("payment button CSS should NOT be critical, got %s", result.Level)
	}
	if result.Level == "hard" {
		t.Errorf("payment button CSS should NOT be hard, got %s", result.Level)
	}
}

func TestRenameDatabaseVariableNotEscalated(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("rename the database variable to dbConn", false, false), cfg)
	if result.Level == "critical" || result.Level == "hard" {
		t.Errorf("rename database variable should not be hard/critical, got %s", result.Level)
	}
}

func TestSecuritySectionInReadmeNotEscalated(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("add a security section in the README", false, false), cfg)
	if result.Level == "critical" || result.Level == "hard" {
		t.Errorf("security section in README should not be hard/critical, got %s", result.Level)
	}
}

func TestConcreteFailureEvidenceFloorHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("3 tests failing with stack trace", false, false), cfg)
	if result.Level != "hard" {
		t.Errorf("tests failing with stack trace should be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestCompileErrorWithRefactorHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("compile error after refactoring three files", false, false), cfg)
	if result.Level != "hard" {
		t.Errorf("compile error after refactor should be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestRepeatedFailureHard(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("the tool call failed twice, same error persists, not working still", false, false), cfg)
	if result.Level != "hard" {
		t.Errorf("repeated failure should be hard, got %s (floors: %v)", result.Level, result.Analysis.Floors)
	}
}

func TestOutageEvidenceCritical(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("production outage, service is down, server log shows crash", false, false), cfg)
	if result.Level != "critical" {
		t.Errorf("production outage with evidence should be critical, got %s (gates: %v)", result.Level, result.Analysis.CriticalGates)
	}
}

func TestDestructiveActionCritical(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("DROP TABLE production data", false, false), cfg)
	if result.Level != "critical" {
		t.Errorf("DROP TABLE should be critical, got %s (gates: %v)", result.Level, result.Analysis.CriticalGates)
	}
}

func TestAnalysisFieldsPopulated(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("debug auth bug in production", false, false), cfg)
	if result.Analysis.Intent == "" {
		t.Error("analysis intent should be populated")
	}
	if result.Analysis.Operation == "" {
		t.Error("analysis operation should be populated")
	}
	if len(result.Analysis.Domains) == 0 {
		t.Error("analysis domains should be populated")
	}
}

func TestEvidenceBasedRoutingTable(t *testing.T) {
	cfg := loadCfg(t)

	cases := []struct {
		text string
		want string
		desc string
	}{
		{"suspicious auth bug", "easy", "topic-only auth stays easy/medium"},
		{"database migration for staging", "easy", "database migration without evidence stays easy/medium"},
		{"explain authentication in one sentence", "easy", "explain read-only stays easy"},
		{"rename the database variable to dbConn", "easy", "variable rename stays easy"},
		{"payment button CSS", "easy", "payment UI stays easy/medium"},
		{"DROP TABLE production data", "critical", "destructive action → critical"},
		{"API key accidentally committed to public git repo, help rotate it", "critical", "secret leak with repo+remediation → critical"},
		{"auth bypass lets users access other customer accounts", "critical", "access violation with impact → critical"},
		{"3 tests failing with stack trace", "hard", "failure evidence → hard"},
		{"compile error after refactoring across files", "hard", "compile error + multi-file → hard"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			result := ClassifySimple(makeInput(tc.text, false, false), cfg)
			if result.Level != tc.want {
				t.Errorf("%s: level = %s, want %s (floors: %v, gates: %v)",
					tc.desc, result.Level, tc.want, result.Analysis.Floors, result.Analysis.CriticalGates)
			}
		})
	}
}

func TestFalsePositiveChecks(t *testing.T) {
	cfg := loadCfg(t)

	t.Run("simple greeting not hard", func(t *testing.T) {
		result := ClassifySimple(makeInput("hi there", false, false), cfg)
		if result.Level != "easy" {
			t.Errorf("greeting should be easy, got %s", result.Level)
		}
	})

	t.Run("simple explanation not hard", func(t *testing.T) {
		result := ClassifySimple(makeInput("what is a closure", false, false), cfg)
		if result.Level == "hard" || result.Level == "critical" {
			t.Errorf("simple explanation should not be hard/critical, got %s", result.Level)
		}
	})

	t.Run("normal code request not critical", func(t *testing.T) {
		result := ClassifySimple(makeInput("write a function to sort an array", false, false), cfg)
		if result.Level == "critical" {
			t.Errorf("normal code request should not be critical, got %s", result.Level)
		}
	})

	t.Run("docs-only request not hard", func(t *testing.T) {
		result := ClassifySimple(makeInput("update the README documentation", false, false), cfg)
		if result.Level == "hard" || result.Level == "critical" {
			t.Errorf("docs-only request should not be hard/critical, got %s", result.Level)
		}
	})

	t.Run("what is API key leak is not critical", func(t *testing.T) {
		result := ClassifySimple(makeInput("what is an API key leak?", false, false), cfg)
		if result.Level == "critical" {
			t.Errorf("what is API key leak should NOT be critical, got %s", result.Level)
		}
	})

	t.Run("coding verb alone stays medium max", func(t *testing.T) {
		result := ClassifySimple(makeInput("write a function", false, false), cfg)
		if result.Level == "hard" || result.Level == "critical" {
			t.Errorf("coding verb alone should not be hard/critical, got %s", result.Level)
		}
	})
}

func TestPolicyReasonsFormat(t *testing.T) {
	cfg := loadCfg(t)
	result := ClassifySimple(makeInput("debug auth bug", false, false), cfg)

	hasIntentReason := false
	for _, r := range result.Reasons {
		if r == "intent:debug" {
			hasIntentReason = true
		}
	}
	if !hasIntentReason {
		t.Errorf("reasons should include 'intent:debug', got %v", result.Reasons)
	}
}
