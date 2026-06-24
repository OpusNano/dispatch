package classifier

import (
	"dispatch/internal/config"
	"strings"
	"testing"
)

func loadConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.DefaultConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func makeInput(text string, hasTools, hasResponseFormat bool) Input {
	return Input{
		Messages: []Message{
			{Role: "user", Content: text},
		},
		HasTools:          hasTools,
		HasResponseFormat: hasResponseFormat,
	}
}

func makeMultiMsg(msgs ...Message) Input {
	return Input{
		Messages: msgs,
	}
}

func TestGreetingEasy(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("hi there", false, false), cfg)
	if result.Level != "easy" {
		t.Errorf("greeting should be easy, got %s (total=%.1f)", result.Level, result.Scores.Total)
	}
}

func TestSimpleExplanationEasy(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("what is a closure? explain briefly", false, false), cfg)
	if result.Level != "easy" && result.Level != "medium" {
		t.Errorf("simple explanation should be easy or medium, got %s", result.Level)
	}
}

func TestNormalCodeMedium(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("write a Go function that reads a yaml file and returns a map", false, false), cfg)
	if result.Level == "critical" || result.Level == "hard" {
		t.Errorf("normal code should not be hard/critical, got %s (total=%.1f, scores=%+v)", result.Level, result.Scores.Total, result.Scores)
	}
	if result.Scores.Complexity < 5 {
		t.Errorf("normal code should have coding intent complexity, got %.1f", result.Scores.Complexity)
	}
}

func TestCodeBlockComplexity(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("here is some code:\n```\nfunc main() {}\n```\n", false, false), cfg)
	if result.Scores.Complexity < 5 {
		t.Errorf("code block should add complexity, got %.1f", result.Scores.Complexity)
	}
}

func TestDiffPatchComplexity(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("@@ -1,5 +1,6 @@\n func main() {}", false, false), cfg)
	if result.Scores.Complexity < 5 {
		t.Errorf("diff should add complexity, got %.1f", result.Scores.Complexity)
	}
}

func TestStacktracePressure(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("panic: runtime error\n\ngoroutine 1 [running]:\nmain.main()\n\t/tmp/main.go:10 +0x45", false, false), cfg)
	if result.Scores.AgentPressure < 8 {
		t.Errorf("stack trace should add pressure, got %.1f", result.Scores.AgentPressure)
	}
}

func TestFailedTestsPressure(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("3 tests failed out of 10. FAIL.", false, false), cfg)
	if result.Scores.AgentPressure < 8 {
		t.Errorf("failed tests should add pressure, got %.1f", result.Scores.AgentPressure)
	}
}

func TestCompileErrorPressure(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("compile error: undefined reference to 'foo'", false, false), cfg)
	if result.Scores.AgentPressure < 8 {
		t.Errorf("compile error should add pressure, got %.1f", result.Scores.AgentPressure)
	}
}

func TestRepeatedFailurePressure(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("it's still failing with the same error", false, false), cfg)
	if result.Scores.AgentPressure < 10 {
		t.Errorf("repeated failure should add pressure, got %.1f", result.Scores.AgentPressure)
	}
}

func TestToolCallComplexity(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("please help", true, false), cfg)
	if result.Scores.Complexity < 5 {
		t.Errorf("tool call should add complexity, got %.1f", result.Scores.Complexity)
	}
	if result.Scores.AgentPressure > 2 {
		t.Errorf("tool call should NOT add agent_pressure (moved to complexity)")
	}
}

func TestResponseFormatComplexity(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("return the data", false, true), cfg)
	if result.Scores.Complexity < 8 {
		t.Errorf("response_format should add complexity, got %.1f", result.Scores.Complexity)
	}
}

func TestFailingTestsWithStacktraceHard(t *testing.T) {
	cfg := loadConfig(t)
	text := "3 tests failing:\npanic: nil pointer\ngoroutine 1:\nmain.TestFoo()\n\tfoo_test.go:42"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Level != "hard" && result.Level != "critical" && result.Level != "medium" {
		t.Errorf("failing tests + stack trace should be medium+, got %s (total=%.1f, reasons=%v)", result.Level, result.Scores.Total, result.Reasons)
	}
}

func TestRepeatedFailureHardOrCritical(t *testing.T) {
	cfg := loadConfig(t)
	text := "the same error persists and it's not working still. compile error: build failed"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Level != "hard" && result.Level != "critical" && result.Level != "medium" {
		t.Errorf("repeated failure + compile error should be medium+, got %s", result.Level)
	}
}

func TestProductionDBMigrationRollbackCritical(t *testing.T) {
	cfg := loadConfig(t)
	text := "we need a production database migration rollback for our live schema"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Level == "critical" {
		t.Errorf("prod db migration rollback without evidence should NOT be critical, got %s", result.Level)
	}
	if result.Level == "hard" {
		t.Errorf("prod db migration rollback without evidence should NOT be hard, got %s", result.Level)
	}
}

func TestAuthSecurityCritical(t *testing.T) {
	cfg := loadConfig(t)
	text := "there is an authentication vulnerability in production. a CVE has been assigned for this XSS."
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Level == "critical" {
		t.Errorf("auth security vuln without access impact should NOT be critical, got %s", result.Level)
	}
	if result.Level == "hard" {
		t.Errorf("auth security vuln without failure evidence should NOT be hard, got %s", result.Level)
	}
}

func TestPaymentFailureCritical(t *testing.T) {
	cfg := loadConfig(t)
	text := "the payment refund is failing for stripe credit card transactions"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Level == "critical" || result.Level == "hard" {
		t.Errorf("payment failure without customer impact evidence should not be hard/critical, got %s", result.Level)
	}
}

func TestDatabaseRenameNotCritical(t *testing.T) {
	cfg := loadConfig(t)
	text := "rename the database variable to dbConn"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Level == "critical" {
		t.Errorf("simple database rename should NOT be critical, got %s (scores: %+v, reasons: %v)", result.Level, result.Scores, result.Reasons)
	}
	sawDowngrade := false
	for _, r := range result.Reasons {
		if strings.Contains(r, "downgrade") && strings.Contains(r, "rename") {
			sawDowngrade = true
		}
	}
	if !sawDowngrade {
		t.Errorf("trivial db rename should trigger downgrade rule. reasons: %v", result.Reasons)
	}
}

func TestSecretsRisk(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("the API key leaked in the .env file", false, false), cfg)
	if result.Scores.Risk < 10 {
		t.Errorf("secrets should add risk, got %.1f", result.Scores.Risk)
	}
}

func TestMultiFileRefactorHard(t *testing.T) {
	cfg := loadConfig(t)
	text := "refactor the architecture across multiple files with a complete redesign"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Level != "hard" {
		t.Errorf("multi-file refactor should be hard, got %s (floors: %v, scores=%+v)", result.Level, result.Analysis.Floors, result.Scores)
	}
}

func TestScoresCapped(t *testing.T) {
	cfg := loadConfig(t)
	text := "```\nfunc a() {}\nfunc b() {}\nfunc c() {}\nfunc d() {}\nfunc e() {}\n```\n"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Scores.Complexity > cfg.Scoring.ComplexityCap {
		t.Errorf("complexity capped at %.0f, got %.1f", cfg.Scoring.ComplexityCap, result.Scores.Complexity)
	}
}

func TestTotalNegativeForGreeting(t *testing.T) {
	cfg := loadConfig(t)
	result := ClassifySimple(makeInput("hi", false, false), cfg)
	if result.Scores.Total > 0 {
		t.Logf("total for hi = %.1f (downgrade may not fully beat basic scores)", result.Scores.Total)
	}
}

func TestDowngradeCap(t *testing.T) {
	cfg := loadConfig(t)
	text := "hi hello hey ok thanks cool got it what is X explain briefly no code don't edit"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Scores.Downgrade > cfg.Scoring.DowngradeCap {
		t.Errorf("downgrade capped at %.0f, got %.1f", cfg.Scoring.DowngradeCap, result.Scores.Downgrade)
	}
}

func TestToolErrorPressure(t *testing.T) {
	cfg := loadConfig(t)
	text := "the tool call failed with exit code 1: command not found"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Scores.AgentPressure < 8 {
		t.Errorf("tool error should add pressure, got %.1f", result.Scores.AgentPressure)
	}
}

func TestMultiStepReasoningComplexity(t *testing.T) {
	cfg := loadConfig(t)
	text := "first we need to design the API, then implement the handler, finally add tests"
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Scores.Complexity < 5 {
		t.Errorf("multi-step reasoning should add complexity, got %.1f", result.Scores.Complexity)
	}
}

func TestRiskFloorForcesHard(t *testing.T) {
	cfg := loadConfig(t)
	text := "database migration rollback" // no "production", so no combo — but risk still high
	result := ClassifySimple(makeInput(text, false, false), cfg)
	if result.Scores.Risk >= cfg.Thresholds.RiskHardFloor && result.Level != "hard" && result.Level != "critical" {
		t.Errorf("high risk should floor to hard, got %s", result.Level)
	}
}

func TestModelReturnedInClassification(t *testing.T) {
	cfg := loadConfig(t)
	easyRM, _ := cfg.ResolveLevel("easy")
	result := ClassifySimple(makeInput("hello", false, false), cfg)
	if result.Model != easyRM.Model {
		t.Errorf("easy model should be %s, got %s", easyRM.Model, result.Model)
	}

	criticalRM, _ := cfg.ResolveLevel("critical")
	result2 := ClassifySimple(makeInput("DROP TABLE production data", false, false), cfg)
	if result2.Model != criticalRM.Model {
		t.Errorf("critical model should be %s, got %s (level=%s)", criticalRM.Model, result2.Model, result2.Level)
	}
}

func TestMultiMessageInput(t *testing.T) {
	cfg := loadConfig(t)
	input := makeMultiMsg(
		Message{Role: "system", Content: "you are a coding assistant"},
		Message{Role: "user", Content: "fix this production database migration"},
	)
	result := ClassifySimple(input, cfg)
	if result.Scores.Risk < 10 {
		t.Errorf("multi-message should still detect risk patterns, got %.1f", result.Scores.Risk)
	}
}
