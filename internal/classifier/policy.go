package classifier

type gateRule struct {
	name      string
	condition func(facts Facts, text string) bool
}

type floorRule struct {
	name      string
	minLevel  string
	condition func(facts Facts, text string) bool
}

var criticalGates = []gateRule{
	{
		name: "concrete_secret_leak",
		condition: func(f Facts, text string) bool {
			if !hasDomain(f, DomainSecrets) {
				return false
			}
			if !f.SecretLeakEvidence {
				return false
			}
			return f.ProductionContext ||
				rePublicRepo.MatchString(text) ||
				reSecretRemediation.MatchString(text) ||
				reKeyMaterial.MatchString(text)
		},
	},
	{
		name: "destructive_data_operation",
		condition: func(f Facts, text string) bool {
			return f.DestructiveAction &&
				(hasDomain(f, DomainDatabase) || f.ProductionContext)
		},
	},
	{
		name: "concrete_access_violation",
		condition: func(f Facts, text string) bool {
			return hasDomain(f, DomainAuth) &&
				reAuthBypass.MatchString(text) &&
				f.AccessImpact
		},
	},
	{
		name: "concrete_transaction_impact",
		condition: func(f Facts, text string) bool {
			return hasDomain(f, DomainPayment) &&
				f.CustomerImpact &&
				rePaymentFail.MatchString(text)
		},
	},
	{
		name: "concrete_outage_evidence",
		condition: func(f Facts, text string) bool {
			return f.ProductionContext && f.OutageEvidence &&
				(hasEvidence(f.Evidence, EvidenceStackTrace) ||
					hasEvidence(f.Evidence, EvidenceLogs) ||
					hasEvidence(f.Evidence, EvidenceToolError))
		},
	},
	{
		name: "irreversible_action",
		condition: func(f Facts, text string) bool {
			return f.DataLossEvidence &&
				(f.ProductionContext ||
					hasDomain(f, DomainDatabase) ||
					f.CustomerImpact)
		},
	},
}

var floorRules = []floorRule{
	{
		name:     "concrete_failure_evidence",
		minLevel: "hard",
		condition: func(f Facts, text string) bool {
			return (hasEvidence(f.Evidence, EvidenceStackTrace) && hasEvidence(f.Evidence, EvidenceTestFailure)) ||
				(hasEvidence(f.Evidence, EvidenceCompileError) && (f.Intent == IntentRefactor || f.Scope == ScopeMultiFile))
		},
	},
	{
		name:     "multi_file_scope",
		minLevel: "hard",
		condition: func(f Facts, text string) bool {
			return (f.Scope == ScopeMultiFile || reMultiFileScope.MatchString(text)) &&
				(f.Intent == IntentRefactor || f.Intent == IntentFixFailure)
		},
	},
	{
		name:     "strict_schema_with_tools",
		minLevel: "hard",
		condition: func(f Facts, text string) bool {
			return hasEvidence(f.Evidence, EvidenceJSONSchema) && hasEvidence(f.Evidence, EvidenceToolCalls)
		},
	},
	{
		name:     "compile_error_alone",
		minLevel: "medium",
		condition: func(f Facts, text string) bool {
			return hasEvidence(f.Evidence, EvidenceCompileError)
		},
	},
	{
		name:     "repeated_failure",
		minLevel: "hard",
		condition: func(f Facts, text string) bool {
			return hasEvidence(f.Evidence, EvidenceRepeatedFailure)
		},
	},
	{
		name:     "architecture_reasoning_complexity",
		minLevel: "hard",
		condition: func(f Facts, text string) bool {
			hasArchitecture := f.Intent == IntentDesignArchitecture || f.Scope == ScopeSystemDesign
			if !hasArchitecture {
				return false
			}
			return f.Scope == ScopeMultiFile || f.Scope == ScopeRepoWide ||
				hasEvidence(f.Evidence, EvidenceCodeBlock) ||
				hasEvidence(f.Evidence, EvidenceDiff) ||
				f.TradeoffAnalysis ||
				reMultipleConstraints.MatchString(text) ||
				reMigrationStrategy.MatchString(text)
		},
	},
}

func isExplainReadOnly(facts Facts) bool {
	if facts.Intent == IntentChat {
		return true
	}
	if (facts.Intent == IntentExplain || facts.Intent == IntentSummarize) &&
		(facts.Operation == OpExplainOnly || facts.Operation == OpReadOnly) {
		return true
	}
	return false
}

func EvaluatePolicy(facts Facts, scoreLevel string, text string) (string, Analysis, []string) {
	analysis := Analysis{
		Intent:     facts.Intent,
		Operation:  facts.Operation,
		Domains:    facts.Domains,
		Scope:      facts.Scope,
		Evidence:   facts.Evidence,
		Risk:       facts.Risk,
		AgentState: facts.AgentState,
	}

	var reasons []string
	reasons = append(reasons, "intent:"+string(facts.Intent))
	reasons = append(reasons, "operation:"+string(facts.Operation))
	for _, d := range facts.Domains {
		reasons = append(reasons, "domain:"+string(d))
	}
	if facts.Scope != ScopeUnknown {
		reasons = append(reasons, "scope:"+string(facts.Scope))
	}
	for _, e := range facts.Evidence {
		if e != EvidenceNone {
			reasons = append(reasons, "evidence:"+string(e))
		}
	}
	if facts.Risk != RiskNone {
		reasons = append(reasons, "risk:"+string(facts.Risk))
	}
	if facts.AgentState != AgentStateFirstAttempt {
		reasons = append(reasons, "agent_state:"+string(facts.AgentState))
	}

	for _, gate := range criticalGates {
		if gate.condition(facts, text) {
			analysis.CriticalGates = append(analysis.CriticalGates, gate.name)
			reasons = append(reasons, "gate:"+gate.name+"=>critical")
			return "critical", analysis, reasons
		}
	}

	floorLevel := "easy"
	for _, rule := range floorRules {
		if rule.condition(facts, text) {
			analysis.Floors = append(analysis.Floors, rule.name+":"+rule.minLevel)
			reasons = append(reasons, "floor:"+rule.name+"=>"+rule.minLevel)
			if levelRank(rule.minLevel) > levelRank(floorLevel) {
				floorLevel = rule.minLevel
			}
		}
	}

	if isExplainReadOnly(facts) {
		if floorLevel != "easy" {
			reasons = append(reasons, "downgrade:explain/read_only removes floor")
		}
		floorLevel = "easy"
	}

	if reToy.MatchString(text) || reLocalDemo.MatchString(text) {
		reduced := reduceLevel(floorLevel)
		if reduced != floorLevel {
			floorLevel = reduced
			reasons = append(reasons, "downgrade:toy/local reduces floor")
		}
	}

	finalLevel := maxLevel(scoreLevel, floorLevel)
	return finalLevel, analysis, reasons
}
