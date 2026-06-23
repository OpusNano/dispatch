package classifier

type Intent string

const (
	IntentChat               Intent = "chat"
	IntentExplain            Intent = "explain"
	IntentSummarize          Intent = "summarize"
	IntentGenerateCode       Intent = "generate_code"
	IntentEditCode           Intent = "edit_code"
	IntentDebug              Intent = "debug"
	IntentFixFailure         Intent = "fix_failure"
	IntentRefactor           Intent = "refactor"
	IntentDesignArchitecture Intent = "design_architecture"
	IntentPlanMigration      Intent = "plan_migration"
	IntentSecurityReview     Intent = "security_review"
	IntentIncidentResponse   Intent = "incident_response"
	IntentUnknown            Intent = "unknown"
)

type Operation string

const (
	OpReadOnly    Operation = "read_only"
	OpExplainOnly Operation = "explain_only"
	OpCreate      Operation = "create"
	OpModify      Operation = "modify"
	OpDelete      Operation = "delete"
	OpMigrate     Operation = "migrate"
	OpRollback    Operation = "rollback"
	OpDeploy      Operation = "deploy"
	OpDebug       Operation = "debug"
	OpVerify      Operation = "verify"
	OpUnknown     Operation = "unknown"
)

type Domain string

const (
	DomainCode       Domain = "code"
	DomainTests      Domain = "tests"
	DomainConfig     Domain = "config"
	DomainDatabase   Domain = "database"
	DomainInfra      Domain = "infra"
	DomainAuth       Domain = "auth"
	DomainPayment    Domain = "payment"
	DomainSecurity   Domain = "security"
	DomainSecrets    Domain = "secrets"
	DomainDeployment Domain = "deployment"
	DomainDocs       Domain = "docs"
	DomainUI         Domain = "ui"
	DomainUnknown    Domain = "unknown"
)

type Scope string

const (
	ScopeTiny         Scope = "tiny"
	ScopeOneFile      Scope = "one_file"
	ScopeMultiFile    Scope = "multi_file"
	ScopeRepoWide     Scope = "repo_wide"
	ScopeSystemDesign Scope = "system_design"
	ScopeUnknown      Scope = "unknown"
)

type Evidence string

const (
	EvidenceCodeBlock       Evidence = "code_block"
	EvidenceDiff            Evidence = "diff"
	EvidencePatch           Evidence = "patch"
	EvidenceStackTrace      Evidence = "stack_trace"
	EvidenceTestFailure     Evidence = "test_failure"
	EvidenceCompileError    Evidence = "compile_error"
	EvidenceToolError       Evidence = "tool_error"
	EvidenceLogs            Evidence = "logs"
	EvidenceJSONSchema      Evidence = "json_schema"
	EvidenceToolCalls       Evidence = "tool_calls"
	EvidenceRepeatedFailure Evidence = "repeated_failure"
	EvidenceNone            Evidence = "none"
)

type Risk string

const (
	RiskNone     Risk = "none"
	RiskLow      Risk = "low"
	RiskMedium   Risk = "medium"
	RiskHigh     Risk = "high"
	RiskCritical Risk = "critical"
)

type AgentState string

const (
	AgentStateFirstAttempt    AgentState = "first_attempt"
	AgentStateToolFailed      AgentState = "tool_failed"
	AgentStateTestsFailed     AgentState = "tests_failed"
	AgentStatePatchFailed     AgentState = "patch_failed"
	AgentStateRepeatedFailure AgentState = "repeated_failure"
	AgentStateStuckLoop       AgentState = "stuck_loop"
	AgentStateUnknown         AgentState = "unknown"
)

type Facts struct {
	Intent     Intent     `json:"intent"`
	Operation  Operation  `json:"operation"`
	Domains    []Domain   `json:"domains"`
	Scope      Scope      `json:"scope"`
	Evidence   []Evidence `json:"evidence"`
	Risk       Risk       `json:"risk"`
	AgentState AgentState `json:"agent_state"`

	DestructiveAction  bool `json:"destructive_action"`
	DataLossEvidence   bool `json:"data_loss_evidence"`
	SecretLeakEvidence bool `json:"secret_leak_evidence"`
	AccessImpact       bool `json:"access_impact"`
	CustomerImpact     bool `json:"customer_impact"`
	OutageEvidence     bool `json:"outage_evidence"`
	ProductionContext  bool `json:"production_context"`
	TradeoffAnalysis   bool `json:"tradeoff_analysis"`
	MultiFileScope     bool `json:"multi_file_scope"`
}

type Analysis struct {
	Intent        Intent     `json:"intent"`
	Operation     Operation  `json:"operation"`
	Domains       []Domain   `json:"domains"`
	Scope         Scope      `json:"scope"`
	Evidence      []Evidence `json:"evidence"`
	Risk          Risk       `json:"risk"`
	AgentState    AgentState `json:"agent_state"`
	Floors        []string   `json:"floors"`
	CriticalGates []string   `json:"critical_gates"`

	Topics               []string `json:"topics,omitempty"`
	OperationObjects     []any    `json:"operation_objects,omitempty"`
	Context              any      `json:"context,omitempty"`
	StructuralComplexity string   `json:"structural_complexity,omitempty"`
	FailureEvidence      []string `json:"failure_evidence,omitempty"`
	Irreversibility      string   `json:"irreversibility,omitempty"`
	TopicEscalation      string   `json:"topic_escalation,omitempty"`
	LengthPolicy         string   `json:"length_policy,omitempty"`
}

func levelRank(level string) int {
	switch level {
	case "easy":
		return 0
	case "medium":
		return 1
	case "hard":
		return 2
	case "critical":
		return 3
	}
	return 0
}

func maxLevel(a, b string) string {
	if levelRank(a) >= levelRank(b) {
		return a
	}
	return b
}

func reduceLevel(level string) string {
	switch level {
	case "critical":
		return "hard"
	case "hard":
		return "medium"
	case "medium":
		return "easy"
	}
	return "easy"
}

func hasDomain(facts Facts, d Domain) bool {
	for _, dom := range facts.Domains {
		if dom == d {
			return true
		}
	}
	return false
}

func hasEvidence(evidence []Evidence, e Evidence) bool {
	for _, ev := range evidence {
		if ev == e {
			return true
		}
	}
	return false
}
