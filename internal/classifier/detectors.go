package classifier

import (
	"regexp"
)

var (
	reGreeting       = regexp.MustCompile(`(?i)^(hi|hello|hey|yo|sup|good (morning|evening)|thanks?|ok|okay|cool|got it)\b`)
	reIncident       = regexp.MustCompile(`(?i)\b(incident|outage|production.*(fail|down|broken|outage))\b`)
	reSecurityReview = regexp.MustCompile(`(?i)\b(vulnerability|CVE\b|exploit|XSS|SQL injection|privilege escalat|security.*(review|audit|vuln|flaw)|bypass)\b`)
	reExplain        = regexp.MustCompile(`(?i)\b(explain|what is|how does|describe|why does|why is|tell me about)\b`)
	reSummarize      = regexp.MustCompile(`(?i)\b(summari[sz]e|tldr|TL;DR|summary of|in short)\b`)
	reMigrate        = regexp.MustCompile(`(?i)\b(migrate|migration|schema change|alter table)\b`)
	reDebug          = regexp.MustCompile(`(?i)\b(debug|trace|diagnose|investigate|suspicious|look into)\b`)
	reFixFailure     = regexp.MustCompile(`(?i)\b(fail|error|broken|crash|bug|not working|doesn.t work)\w*\b`)
	reRefactor       = regexp.MustCompile(`(?i)\b(refactor|restructure|clean up|reorganize)\w*\b`)
	reArchitecture   = regexp.MustCompile(`(?i)\b(architecture|system design|design.*(api|service|system|protocol)|monorepo|polyrepo)\b`)
	reGenerateCode   = regexp.MustCompile(`(?i)\b(write|create|generate|implement|build|add)\b`)
	reEditCode       = regexp.MustCompile(`(?i)\b(edit|change|update|modify|fix|patch|rename|move)\b`)

	reRollback = regexp.MustCompile(`(?i)\b(rollback|revert|undo|roll back)\b`)
	reDeployOp = regexp.MustCompile(`(?i)\b(deploy|release|publish|ship|roll out)\b`)
	reDelete   = regexp.MustCompile(`(?i)\b(delete|remove|drop|purge)\b`)
	reVerify   = regexp.MustCompile(`(?i)\b(test|verify|validate|check|run tests)\b`)
	reReadOnly = regexp.MustCompile(`(?i)\b(read|show|list|display|print)\b`)

	reCodeDomain     = regexp.MustCompile(`(?i)\b(function|class|method|variable|module|handler|endpoint|api|service|controller|code|program|script|interface|struct|type)\b`)
	reTestsDomain    = regexp.MustCompile(`(?i)\b(tests?|spec|mock|fixture|testing|unittest|integration test)\b`)
	reConfigDomain   = regexp.MustCompile(`(?i)\b(config|yaml|toml|env file|settings|configuration)\b`)
	reDatabaseDomain = regexp.MustCompile(`(?i)\b(database|db\b|table|column|sql|schema|alter table|index|query|migration)\b`)
	reInfraDomain    = regexp.MustCompile(`(?i)\b(docker|kubernetes|terraform|server|infrastructure|cluster|node|pod)\b`)
	reAuthDomain     = regexp.MustCompile(`(?i)\bauth(entication|orization)?\b|\b(login|password|token|session|oauth|jwt|authn|authz)\b`)
	rePaymentDomain  = regexp.MustCompile(`(?i)\b(payment|billing|charge|refund|stripe|credit card|transaction|invoice|checkout)\b`)
	reSecurityDomain = regexp.MustCompile(`(?i)\bsecurity\b|\b(vulnerability|CVE|XSS|injection|exploit|privilege escalat|CSRF|clickjacking)\b`)
	reSecretsDomain  = regexp.MustCompile(`(?i)\b(secret|api[ _-]?key|private key|credential|token leak|password leak)\b|\.env\b`)
	reDeployDomain   = regexp.MustCompile(`(?i)\b(deploy|release|production|staging|CI.CD|pipeline|rollout)\b`)
	reDocsDomain     = regexp.MustCompile(`(?i)\b(readme|documentation|docs|markdown|wiki)\b`)
	reUIDomain       = regexp.MustCompile(`(?i)\b(css|html|button|frontend|ui component|style|layout|widget|tailwind|bootstrap)\b`)

	reTinyScope         = regexp.MustCompile(`(?i)\b(rename|one sentence|briefly|tldr|short answer|simple|trivial|just)\b`)
	reOneFileScope      = regexp.MustCompile(`(?i)\b(this file|the file|edit the|fix the|update the|change the)\b`)
	reMultiFileScope    = regexp.MustCompile(`(?i)\b(multiple files|across files|several files|multi.file|cross.file)\b`)
	reRepoWideScope     = regexp.MustCompile(`(?i)\b(entire repo|whole project|all files|monorepo|every file)\b`)
	reSystemDesignScope = regexp.MustCompile(`(?i)\b(architecture|system design|design.*(api|service|system|protocol))\b`)

	reCodeBlock     = regexp.MustCompile("(?s)`[^`]*`")
	reDiffEvidence  = regexp.MustCompile(`(?m)^(@@|[\+\-]{3} |diff --git)`)
	rePatchEvidence = regexp.MustCompile(`(?i)\bpatch\b|\bdid not apply\b`)
	reStackTrace    = regexp.MustCompile(`(?im)\s+at .+\(.+:\d+\)|panic:|goroutine \d+|Traceback|stack trace`)
	reTestFailure   = regexp.MustCompile(`(?i)\bFAIL(URE)?\b|tests?.*(failed|failing)|\d+ failing`)
	reCompileError  = regexp.MustCompile(`(?i)\b(compile error|syntax error|cannot find|undefined (reference|symbol)|build failed)\b`)
	reToolError     = regexp.MustCompile(`(?i)\btool (call )?(failed|error)|exit code [1-9]|command not found\b`)
	reLogsEvidence  = regexp.MustCompile(`(?i)\b(log|console output|stderr|stdout|server log)\b`)
	reRepeatedFail  = regexp.MustCompile(`(?i)\b(still failing|same error|again.*error|error persists|not working still|keeps failing|repeatedly)\b`)

	reProduction = regexp.MustCompile(`(?i)\b(production|prod\b|live (system|traffic|environment)|customer.facing)\b`)
	reStaging    = regexp.MustCompile(`(?i)\b(staging|stage environment|pre.prod)\b`)
	reDataLoss   = regexp.MustCompile(`(?i)\b(data loss|irreversible|drop table|delete all|truncate|wipe|destroy)\b`)

	reSecretLeak     = regexp.MustCompile(`(?i)\b(committed|leaked|exposed|pushed)\b|in.*(repo|git)`)
	rePaymentFail    = regexp.MustCompile(`(?i)\b(fail|error|broken|crash|chargeback|dispute)\w*\b`)
	reProductionFail = regexp.MustCompile(`(?i)\b(fail|down|broken|outage|error|crash)\w*\b`)
	reAuthBypass     = regexp.MustCompile(`(?i)\b(bypass|exploit|vulnerab)\b`)
	rePatchFailed    = regexp.MustCompile(`(?i)\b(did not apply|not apply|patch.*reject|patch.*fail)\b`)

	reToy       = regexp.MustCompile(`(?i)\b(toy|example|demo|sample|sandbox|playground)\b`)
	reLocalDemo = regexp.MustCompile(`(?i)\b(local|development|dev environment|sandbox|my machine)\b`)

	reDestructiveAction   = regexp.MustCompile(`(?i)\b(drop table|drop column|truncate|delete all|wipe|destroy)\b`)
	reDataLossEvidence    = regexp.MustCompile(`(?i)\b(data loss|irreversible|cannot be undone|permanent loss)\b`)
	reSecretLeakEvidence  = regexp.MustCompile(`(?i)\b(committed|leaked|exposed|pushed)\b.*\b(secret|api.?key|private.?key|credential|password|token)\b|\b(secret|api.?key|private.?key|credential|password|token)\b.*\b(committed|leaked|exposed|pushed)\b`)
	rePublicRepo          = regexp.MustCompile(`(?i)\b(public.?repo|git.?repo|github|gitlab|bitbucket|pushed.*repo|committed.*repo)\b`)
	reSecretRemediation   = regexp.MustCompile(`(?i)\b(rotate|revoke|invalidate|remove.*key|change.*password|reset.*credential)\b`)
	reKeyMaterial         = regexp.MustCompile(`(?i)-----\s*BEGIN.*PRIVATE KEY|-----\s*BEGIN.*RSA|-----\s*BEGIN.*OPENSSH|sk-[a-zA-Z0-9]{20,}|AKIA[A-Z0-9]{16}`)
	reTradeoffAnalysis    = regexp.MustCompile(`(?i)\b(tradeoffs?|trade.?off|pros?.*cons?|advantage.*disadvantage|versus|vs\.?|better.*worse|cost.*benefit)\b`)
	reMultipleConstraints = regexp.MustCompile(`(?i)\b(constraint|requirement|must.*also|both.*and.*need|multiple.*requirement)\b`)
	reMigrationStrategy   = regexp.MustCompile(`(?i)\b(strategy|plan.*step|first.*then.*finally|phase \d|stage \d|migration.*plan|rollout.*plan)\b`)
	reAccessImpact        = regexp.MustCompile(`(?i)\b(access.*(other|someone).*(account|data)|customers?.*(affected|impacted|losing|lost)|users?.*(access|see).*(other|someone))\b`)
	reCustomerImpact      = regexp.MustCompile(`(?i)\b(customers?|users?|clients?)\b.*\b(affected|impacted|losing|lost|cannot|unable)\b`)
	reOutageEvidence      = regexp.MustCompile(`(?i)\b(outage|down|crash.?loop|not responding|unreachable)\b`)
)

func detectIntent(text string) Intent {
	if reGreeting.MatchString(text) {
		return IntentChat
	}
	if reIncident.MatchString(text) {
		return IntentIncidentResponse
	}
	if reSecurityReview.MatchString(text) {
		return IntentSecurityReview
	}
	if reExplain.MatchString(text) {
		return IntentExplain
	}
	if reSummarize.MatchString(text) {
		return IntentSummarize
	}
	if reMigrate.MatchString(text) {
		return IntentPlanMigration
	}
	if reDebug.MatchString(text) {
		return IntentDebug
	}
	if reRefactor.MatchString(text) {
		return IntentRefactor
	}
	if reFixFailure.MatchString(text) {
		return IntentFixFailure
	}
	if reArchitecture.MatchString(text) {
		return IntentDesignArchitecture
	}
	if reGenerateCode.MatchString(text) {
		return IntentGenerateCode
	}
	if reEditCode.MatchString(text) {
		return IntentEditCode
	}
	return IntentUnknown
}

func detectOperation(text string) Operation {
	if reRollback.MatchString(text) {
		return OpRollback
	}
	if reDeployOp.MatchString(text) {
		return OpDeploy
	}
	if reMigrate.MatchString(text) {
		return OpMigrate
	}
	if reDelete.MatchString(text) {
		return OpDelete
	}
	if reExplain.MatchString(text) {
		return OpExplainOnly
	}
	if reDebug.MatchString(text) {
		return OpDebug
	}
	if reGenerateCode.MatchString(text) {
		return OpCreate
	}
	if reVerify.MatchString(text) {
		return OpVerify
	}
	if reEditCode.MatchString(text) {
		return OpModify
	}
	if reReadOnly.MatchString(text) {
		return OpReadOnly
	}
	return OpUnknown
}

func detectDomains(text string) []Domain {
	var domains []Domain
	if reCodeDomain.MatchString(text) {
		domains = append(domains, DomainCode)
	}
	if reTestsDomain.MatchString(text) {
		domains = append(domains, DomainTests)
	}
	if reConfigDomain.MatchString(text) {
		domains = append(domains, DomainConfig)
	}
	if reDatabaseDomain.MatchString(text) {
		domains = append(domains, DomainDatabase)
	}
	if reInfraDomain.MatchString(text) {
		domains = append(domains, DomainInfra)
	}
	if reAuthDomain.MatchString(text) {
		domains = append(domains, DomainAuth)
	}
	if rePaymentDomain.MatchString(text) {
		domains = append(domains, DomainPayment)
	}
	if reSecurityDomain.MatchString(text) {
		domains = append(domains, DomainSecurity)
	}
	if reSecretsDomain.MatchString(text) {
		domains = append(domains, DomainSecrets)
	}
	if reDeployDomain.MatchString(text) {
		domains = append(domains, DomainDeployment)
	}
	if reDocsDomain.MatchString(text) {
		domains = append(domains, DomainDocs)
	}
	if reUIDomain.MatchString(text) {
		domains = append(domains, DomainUI)
	}
	if len(domains) == 0 {
		domains = append(domains, DomainUnknown)
	}
	return domains
}

func detectScope(text string) Scope {
	if reRepoWideScope.MatchString(text) {
		return ScopeRepoWide
	}
	if reSystemDesignScope.MatchString(text) {
		return ScopeSystemDesign
	}
	if reMultiFileScope.MatchString(text) {
		return ScopeMultiFile
	}
	if reOneFileScope.MatchString(text) {
		return ScopeOneFile
	}
	if reTinyScope.MatchString(text) {
		return ScopeTiny
	}
	return ScopeUnknown
}

func detectEvidence(text string, hasTools, hasResponseFormat bool) []Evidence {
	var evidence []Evidence
	if reCodeBlock.MatchString(text) {
		evidence = append(evidence, EvidenceCodeBlock)
	}
	if reDiffEvidence.MatchString(text) {
		evidence = append(evidence, EvidenceDiff)
	}
	if rePatchEvidence.MatchString(text) {
		evidence = append(evidence, EvidencePatch)
	}
	if reStackTrace.MatchString(text) {
		evidence = append(evidence, EvidenceStackTrace)
	}
	if reTestFailure.MatchString(text) {
		evidence = append(evidence, EvidenceTestFailure)
	}
	if reCompileError.MatchString(text) {
		evidence = append(evidence, EvidenceCompileError)
	}
	if reToolError.MatchString(text) {
		evidence = append(evidence, EvidenceToolError)
	}
	if reLogsEvidence.MatchString(text) {
		evidence = append(evidence, EvidenceLogs)
	}
	if reRepeatedFail.MatchString(text) {
		evidence = append(evidence, EvidenceRepeatedFailure)
	}
	if hasResponseFormat {
		evidence = append(evidence, EvidenceJSONSchema)
	}
	if hasTools {
		evidence = append(evidence, EvidenceToolCalls)
	}
	if len(evidence) == 0 {
		evidence = append(evidence, EvidenceNone)
	}
	return evidence
}

func detectRisk(text string, domains []Domain, intent Intent) Risk {
	hasProd := reProduction.MatchString(text)
	hasDataLoss := reDataLoss.MatchString(text)

	hasAuth := containsDomain(domains, DomainAuth)
	hasSecurity := containsDomain(domains, DomainSecurity)
	hasSecrets := containsDomain(domains, DomainSecrets)
	hasDatabase := containsDomain(domains, DomainDatabase)
	hasPayment := containsDomain(domains, DomainPayment)

	if hasDataLoss {
		return RiskCritical
	}
	if hasProd && (hasAuth || hasSecurity || hasSecrets || hasDatabase) {
		return RiskCritical
	}
	if hasProd && hasPayment {
		return RiskCritical
	}
	if hasSecrets && reSecretLeak.MatchString(text) {
		return RiskCritical
	}
	if hasPayment && rePaymentFail.MatchString(text) {
		return RiskCritical
	}
	if hasProd {
		return RiskHigh
	}
	if hasAuth || hasSecurity || hasSecrets {
		return RiskHigh
	}
	if hasDatabase && intent == IntentPlanMigration {
		return RiskHigh
	}
	if hasPayment {
		return RiskHigh
	}
	if containsDomain(domains, DomainDatabase) {
		return RiskMedium
	}
	if containsDomain(domains, DomainTests) || containsDomain(domains, DomainConfig) {
		return RiskLow
	}
	return RiskNone
}

func detectAgentState(text string, evidence []Evidence) AgentState {
	hasRepeated := hasEvidence(evidence, EvidenceRepeatedFailure)
	hasToolError := hasEvidence(evidence, EvidenceToolError)
	hasTestFailure := hasEvidence(evidence, EvidenceTestFailure)
	hasPatch := hasEvidence(evidence, EvidencePatch)
	hasCompileError := hasEvidence(evidence, EvidenceCompileError)

	if hasRepeated && (hasToolError || hasTestFailure || hasCompileError) {
		return AgentStateStuckLoop
	}
	if hasRepeated {
		return AgentStateRepeatedFailure
	}
	if hasPatch && rePatchFailed.MatchString(text) {
		return AgentStatePatchFailed
	}
	if hasTestFailure {
		return AgentStateTestsFailed
	}
	if hasToolError {
		return AgentStateToolFailed
	}
	return AgentStateFirstAttempt
}

func ExtractFacts(text string, hasTools, hasResponseFormat bool) Facts {
	intent := detectIntent(text)
	operation := detectOperation(text)
	domains := detectDomains(text)
	scope := detectScope(text)
	evidence := detectEvidence(text, hasTools, hasResponseFormat)
	risk := detectRisk(text, domains, intent)
	agentState := detectAgentState(text, evidence)

	return Facts{
		Intent:     intent,
		Operation:  operation,
		Domains:    domains,
		Scope:      scope,
		Evidence:   evidence,
		Risk:       risk,
		AgentState: agentState,

		DestructiveAction:  reDestructiveAction.MatchString(text),
		DataLossEvidence:   reDataLossEvidence.MatchString(text),
		SecretLeakEvidence: reSecretLeakEvidence.MatchString(text),
		AccessImpact:       reAccessImpact.MatchString(text),
		CustomerImpact:     reCustomerImpact.MatchString(text),
		OutageEvidence:     reOutageEvidence.MatchString(text),
		ProductionContext:  reProduction.MatchString(text),
		TradeoffAnalysis:   reTradeoffAnalysis.MatchString(text),
		MultiFileScope:     scope == ScopeMultiFile,
	}
}

func containsDomain(domains []Domain, d Domain) bool {
	for _, dom := range domains {
		if dom == d {
			return true
		}
	}
	return false
}
