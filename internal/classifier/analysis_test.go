package classifier

import (
	"testing"
)

func TestDetectIntent(t *testing.T) {
	tests := []struct {
		text string
		want Intent
	}{
		{"hi there", IntentChat},
		{"hello world", IntentChat},
		{"ok got it", IntentChat},
		{"explain authentication in one sentence", IntentExplain},
		{"what is a closure", IntentExplain},
		{"describe how the auth flow works", IntentExplain},
		{"summarize this document", IntentSummarize},
		{"tldr of the changes", IntentSummarize},
		{"write a function to sort an array", IntentGenerateCode},
		{"create a new endpoint", IntentGenerateCode},
		{"edit the config file", IntentEditCode},
		{"rename the variable to dbConn", IntentEditCode},
		{"debug auth bug", IntentDebug},
		{"suspicious auth bug", IntentDebug},
		{"investigate the login flow", IntentDebug},
		{"fix this compile error", IntentFixFailure},
		{"the tests are failing", IntentFixFailure},
		{"production deploy is failing", IntentFixFailure},
		{"compile error after refactor", IntentRefactor},
		{"auth bypass vulnerability", IntentSecurityReview},
		{"CVE-2024-1234 in the auth module", IntentSecurityReview},
		{"refactor the architecture", IntentRefactor},
		{"restructure the codebase", IntentRefactor},
		{"design the system architecture", IntentDesignArchitecture},
		{"should we use a monorepo", IntentDesignArchitecture},
		{"database migration plan", IntentPlanMigration},
		{"alter table to add column", IntentPlanMigration},
		{"production outage", IntentIncidentResponse},
		{"please help me", IntentUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := detectIntent(tt.text)
			if got != tt.want {
				t.Errorf("detectIntent(%q) = %s, want %s", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectOperation(t *testing.T) {
	tests := []struct {
		text string
		want Operation
	}{
		{"rollback the last migration", OpRollback},
		{"revert the changes", OpRollback},
		{"deploy to production", OpDeploy},
		{"release version 2.0", OpDeploy},
		{"migrate the database", OpMigrate},
		{"alter table users", OpMigrate},
		{"delete the old records", OpDelete},
		{"remove unused imports", OpDelete},
		{"explain how auth works", OpExplainOnly},
		{"what is a closure", OpExplainOnly},
		{"debug the login issue", OpDebug},
		{"investigate the error", OpDebug},
		{"write a function", OpCreate},
		{"build a new service", OpCreate},
		{"test the endpoint", OpVerify},
		{"validate the output", OpVerify},
		{"edit the config", OpModify},
		{"fix the bug", OpModify},
		{"rename the variable", OpModify},
		{"show me the logs", OpReadOnly},
		{"list all files", OpReadOnly},
		{"please help", OpUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := detectOperation(tt.text)
			if got != tt.want {
				t.Errorf("detectOperation(%q) = %s, want %s", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectDomains(t *testing.T) {
	tests := []struct {
		text     string
		mustHave []Domain
		mustNot  []Domain
	}{
		{"write a function", []Domain{DomainCode}, nil},
		{"run the tests", []Domain{DomainTests}, nil},
		{"edit the config file", []Domain{DomainConfig}, nil},
		{"database migration", []Domain{DomainDatabase}, nil},
		{"docker container", []Domain{DomainInfra}, nil},
		{"authentication vulnerability", []Domain{DomainAuth, DomainSecurity}, nil},
		{"payment refund", []Domain{DomainPayment}, nil},
		{"security audit", []Domain{DomainSecurity}, nil},
		{"API key leaked", []Domain{DomainSecrets}, nil},
		{"deploy to production", []Domain{DomainDeployment}, nil},
		{"update the README", []Domain{DomainDocs}, nil},
		{"change the CSS", []Domain{DomainUI}, nil},
		{"auth bug in production", []Domain{DomainAuth, DomainDeployment}, nil},
		{"hello world", []Domain{DomainUnknown}, []Domain{DomainCode, DomainAuth}},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := detectDomains(tt.text)
			for _, d := range tt.mustHave {
				if !containsDomain(got, d) {
					t.Errorf("detectDomains(%q) missing domain %s, got %v", tt.text, d, got)
				}
			}
			for _, d := range tt.mustNot {
				if containsDomain(got, d) {
					t.Errorf("detectDomains(%q) should NOT include domain %s, got %v", tt.text, d, got)
				}
			}
		})
	}
}

func TestDetectScope(t *testing.T) {
	tests := []struct {
		text string
		want Scope
	}{
		{"rename the variable", ScopeTiny},
		{"explain in one sentence", ScopeTiny},
		{"edit this file", ScopeOneFile},
		{"fix the bug", ScopeOneFile},
		{"refactor across multiple files", ScopeMultiFile},
		{"update all files in the repo", ScopeRepoWide},
		{"design the system architecture", ScopeSystemDesign},
		{"hello world", ScopeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := detectScope(tt.text)
			if got != tt.want {
				t.Errorf("detectScope(%q) = %s, want %s", tt.text, got, tt.want)
			}
		})
	}
}

func TestDetectEvidence(t *testing.T) {
	tests := []struct {
		text     string
		hasTools bool
		hasRF    bool
		mustHave []Evidence
	}{
		{"here is code:\n```\nfunc main(){}\n```\n", false, false, []Evidence{EvidenceCodeBlock}},
		{"@@ -1,3 +1,4 @@\n-old\n+new", false, false, []Evidence{EvidenceDiff}},
		{"the patch did not apply", false, false, []Evidence{EvidencePatch}},
		{"panic: runtime error\ngoroutine 1 [running]:\nmain.go:10", false, false, []Evidence{EvidenceStackTrace}},
		{"3 tests failed", false, false, []Evidence{EvidenceTestFailure}},
		{"compile error: undefined reference", false, false, []Evidence{EvidenceCompileError}},
		{"tool call failed with exit code 1", false, false, []Evidence{EvidenceToolError}},
		{"still failing with same error", false, false, []Evidence{EvidenceRepeatedFailure}},
		{"test with tools", true, false, []Evidence{EvidenceToolCalls}},
		{"response format", false, true, []Evidence{EvidenceJSONSchema}},
		{"hello world", false, false, []Evidence{EvidenceNone}},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := detectEvidence(tt.text, tt.hasTools, tt.hasRF)
			for _, e := range tt.mustHave {
				if !hasEvidence(got, e) {
					t.Errorf("detectEvidence(%q) missing evidence %s, got %v", tt.text, e, got)
				}
			}
		})
	}
}

func TestDetectRisk(t *testing.T) {
	tests := []struct {
		text    string
		domains []Domain
		intent  Intent
		want    Risk
	}{
		{"hello world", []Domain{}, IntentChat, RiskNone},
		{"production database migration", []Domain{DomainDatabase}, IntentPlanMigration, RiskCritical},
		{"auth bypass in production", []Domain{DomainAuth, DomainSecurity}, IntentSecurityReview, RiskCritical},
		{"API key committed", []Domain{DomainSecrets}, IntentUnknown, RiskCritical},
		{"payment refund failing", []Domain{DomainPayment}, IntentFixFailure, RiskCritical},
		{"data loss risk", []Domain{DomainDatabase}, IntentUnknown, RiskCritical},
		{"deploy to production", []Domain{DomainDeployment}, IntentGenerateCode, RiskHigh},
		{"suspicious auth bug", []Domain{DomainAuth}, IntentDebug, RiskHigh},
		{"database migration for staging", []Domain{DomainDatabase}, IntentPlanMigration, RiskHigh},
		{"payment button CSS", []Domain{DomainPayment, DomainUI}, IntentUnknown, RiskHigh},
		{"staging environment setup", []Domain{DomainInfra}, IntentGenerateCode, RiskNone},
		{"run the tests", []Domain{DomainTests}, IntentGenerateCode, RiskLow},
		{"edit the config", []Domain{DomainConfig}, IntentEditCode, RiskLow},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := detectRisk(tt.text, tt.domains, tt.intent)
			if got != tt.want {
				t.Errorf("detectRisk(%q, %v, %s) = %s, want %s", tt.text, tt.domains, tt.intent, got, tt.want)
			}
		})
	}
}

func TestDetectAgentState(t *testing.T) {
	tests := []struct {
		text     string
		evidence []Evidence
		want     AgentState
	}{
		{"hello world", []Evidence{EvidenceNone}, AgentStateFirstAttempt},
		{"tool call failed", []Evidence{EvidenceToolError}, AgentStateToolFailed},
		{"3 tests failed", []Evidence{EvidenceTestFailure}, AgentStateTestsFailed},
		{"the patch did not apply", []Evidence{EvidencePatch}, AgentStatePatchFailed},
		{"same error persists", []Evidence{EvidenceRepeatedFailure}, AgentStateRepeatedFailure},
		{"still failing with same error and tool failed", []Evidence{EvidenceRepeatedFailure, EvidenceToolError}, AgentStateStuckLoop},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := detectAgentState(tt.text, tt.evidence)
			if got != tt.want {
				t.Errorf("detectAgentState(%q, %v) = %s, want %s", tt.text, tt.evidence, got, tt.want)
			}
		})
	}
}

func TestExtractFactsEndToEnd(t *testing.T) {
	text := "debug the auth bug in production - the tool call failed twice with the same error"
	facts := ExtractFacts(text, true, false)

	if facts.Intent != IntentDebug && facts.Intent != IntentFixFailure && facts.Intent != IntentIncidentResponse {
		t.Errorf("intent = %s, expected debug/fix_failure/incident_response", facts.Intent)
	}
	if !hasDomain(facts, DomainAuth) {
		t.Errorf("domains = %v, expected auth", facts.Domains)
	}
	if !hasDomain(facts, DomainDeployment) {
		t.Errorf("domains = %v, expected deployment", facts.Domains)
	}
	if facts.AgentState != AgentStateStuckLoop && facts.AgentState != AgentStateRepeatedFailure {
		t.Errorf("agent_state = %s, expected stuck_loop or repeated_failure", facts.AgentState)
	}
}
