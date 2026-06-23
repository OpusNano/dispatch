package classifier

import (
	"strings"

	"dispatch/internal/config"
)

type OpObjRole struct {
	Operation        string   `json:"operation"`
	Object           string   `json:"object"`
	Role             string   `json:"role"`
	EvidenceStrength string   `json:"evidence_strength"`
	MinLevel         string   `json:"min_level"`
	Reason           string   `json:"reason"`
	EvidenceMatched  []string `json:"evidence_matched,omitempty"`
}

type operationMatch struct {
	word     string
	position int
	op       string
}

type objectMatch struct {
	word     string
	position int
	concept  string
}

func ExtractOperationObjectRoles(facts Facts, text string, cfg *config.Config) []OpObjRole {
	if cfg.Intelligence == nil || !cfg.Intelligence.Enabled {
		return nil
	}

	rules := cfg.Intelligence.OperationObject.Rules
	if len(rules) == 0 {
		return nil
	}

	proximityWindow := cfg.Intelligence.OperationObject.ProximityWindow
	if proximityWindow <= 0 {
		proximityWindow = 5
	}

	tokens := tokenize(text)
	ops := findOperations(tokens, rules)
	objs := findObjects(tokens, rules, cfg.Intelligence.Concepts)

	var results []OpObjRole
	for _, opMatch := range ops {
		for _, objMatch := range objs {
			dist := abs(opMatch.position - objMatch.position)
			if dist > proximityWindow {
				continue
			}

			for _, rule := range rules {
				if !operationMatches(opMatch.op, rule.Operation) {
					continue
				}
				if !objectMatches(objMatch.concept, rule.Object) {
					continue
				}

				role := classifyRole(tokens, opMatch.position, objMatch.position, facts)
				if role == "" {
					role = classifyRoleFromConcepts(objMatch.concept, facts)
				}

				evidenceStrength := "low"
				var evidenceMatched []string
				if len(rule.RequiresEvidence) > 0 {
					evidenceMatched, evidenceStrength = checkOpObjEvidence(rule.RequiresEvidence, facts, text)
				}

				minLevel := rule.MinLevel
				if evidenceStrength == "high" && levelRank(minLevel) < levelRank("hard") {
					minLevel = "hard"
				}

				results = append(results, OpObjRole{
					Operation:        opMatch.op,
					Object:           objMatch.concept,
					Role:             role,
					EvidenceStrength: evidenceStrength,
					MinLevel:         minLevel,
					Reason:           rule.Reason,
					EvidenceMatched:  evidenceMatched,
				})
			}
		}
	}
	return results
}

func tokenize(text string) []string {
	normalized := strings.ToLower(text)
	var result []string
	var current strings.Builder
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			result = append(result, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func findOperations(tokens []string, rules []config.OpObjRuleDef) []operationMatch {
	opSet := make(map[string]bool)
	for _, r := range rules {
		opSet[r.Operation] = true
	}

	var results []operationMatch
	for i, tok := range tokens {
		for op := range opSet {
			if tok == op || strings.HasPrefix(tok, op) || strings.HasSuffix(tok, op) {
				results = append(results, operationMatch{word: tok, position: i, op: op})
			}
		}
	}
	return results
}

func findObjects(tokens []string, rules []config.OpObjRuleDef, concepts []config.ConceptDef) []objectMatch {
	objSet := make(map[string]bool)
	for _, r := range rules {
		objSet[r.Object] = true
	}

	conceptMap := buildConceptMap(concepts)

	var results []objectMatch
	for i, tok := range tokens {
		for obj := range objSet {
			if tok == obj {
				results = append(results, objectMatch{word: tok, position: i, concept: obj})
				continue
			}
			if aliases, ok := conceptMap[obj]; ok {
				for _, alias := range aliases {
					if tok == alias {
						results = append(results, objectMatch{word: tok, position: i, concept: obj})
						break
					}
				}
			}
		}
	}
	return results
}

func buildConceptMap(concepts []config.ConceptDef) map[string][]string {
	m := make(map[string][]string)
	for _, c := range concepts {
		m[c.Name] = c.Aliases
	}
	return m
}

func operationMatches(found, rule string) bool {
	return found == rule
}

func objectMatches(found, rule string) bool {
	if rule == "any" {
		return true
	}
	return found == rule
}

func classifyRole(tokens []string, opPos, objPos int, facts Facts) string {
	minPos := opPos
	maxPos := objPos
	if objPos < opPos {
		minPos = objPos
		maxPos = opPos
	}

	for i := maxPos + 1; i < len(tokens) && i <= maxPos+3; i++ {
		switch tokens[i] {
		case "button", "css", "html", "style", "layout", "color", "font", "tailwind", "ui":
			return "ui_component"
		case "variable", "var", "const", "let", "name":
			return "variable_name"
		case "readme", "documentation", "docs", "markdown", "wiki":
			return "readme_or_docs"
		case "log", "console", "stderr", "stdout":
			return "log_blob"
		case "transcript", "conversation", "chat":
			return "transcript_blob"
		case "typo", "text", "label", "string", "message", "copy":
			return "simple_copy_change"
		case "toy", "example", "demo", "sample", "sandbox", "local":
			return "local_demo"
		case "refund", "charge", "stripe", "billing", "checkout", "chargeback":
			return "transaction"
		case "bypass", "exploit", "vulnerability", "vuln":
			return "security_boundary"
		case "schema", "ddl":
			return "schema"
		case "production", "prod", "live", "customer-facing", "deploy":
			return "production_system"
		}
	}

	for i := minPos - 1; i >= 0 && i >= minPos-3; i-- {
		switch tokens[i] {
		case "button", "css", "html", "style", "layout", "color", "font":
			return "ui_component"
		case "variable", "var", "const", "let":
			return "variable_name"
		case "readme", "documentation", "docs":
			return "readme_or_docs"
		case "toy", "example", "demo", "sample", "sandbox", "local":
			return "local_demo"
		case "typo", "text", "label", "string", "copy":
			return "simple_copy_change"
		case "refund", "charge", "stripe", "billing", "checkout":
			return "transaction"
		case "bypass", "exploit", "vulnerability", "vuln":
			return "security_boundary"
		case "schema":
			return "schema"
		case "production", "prod", "live", "customer-facing":
			return "production_system"
		}
	}
	return ""
}

func classifyRoleFromConcepts(concept string, facts Facts) string {
	switch concept {
	case "auth":
		if facts.Intent == IntentExplain || facts.Intent == IntentSummarize {
			return "concept"
		}
		if facts.Scope == ScopeTiny || facts.Scope == ScopeOneFile {
			return "ui_component"
		}
		return "security_boundary"
	case "payment":
		return "transaction"
	case "database", "database_migration", "database_rollback":
		return "data_state"
	case "deployment", "production":
		return "production_system"
	case "security":
		return "security_boundary"
	case "secrets":
		return "secret_material"
	case "ui":
		return "ui_component"
	case "docs":
		return "readme_or_docs"
	}
	return "business_logic"
}

func checkOpObjEvidence(required []string, facts Facts, text string) ([]string, string) {
	var matched []string
	allCritical := true

	for _, req := range required {
		found := false
		switch req {
		case "stack_trace":
			found = hasEvidence(facts.Evidence, EvidenceStackTrace)
		case "test_failure":
			found = hasEvidence(facts.Evidence, EvidenceTestFailure)
		case "compile_error":
			found = hasEvidence(facts.Evidence, EvidenceCompileError)
		case "tool_error":
			found = hasEvidence(facts.Evidence, EvidenceToolError)
		case "repeated_failure":
			found = hasEvidence(facts.Evidence, EvidenceRepeatedFailure)
		case "json_schema":
			found = hasEvidence(facts.Evidence, EvidenceJSONSchema)
		case "tool_calls":
			found = hasEvidence(facts.Evidence, EvidenceToolCalls)
		case "code_block":
			found = hasEvidence(facts.Evidence, EvidenceCodeBlock)
		case "diff":
			found = hasEvidence(facts.Evidence, EvidenceDiff)
		case "logs":
			found = hasEvidence(facts.Evidence, EvidenceLogs)
		case "multi_file_scope":
			found = facts.Scope == ScopeMultiFile
		case "destructive_action":
			found = facts.DestructiveAction
		case "secret_leak_evidence":
			found = facts.SecretLeakEvidence
		case "access_impact":
			found = facts.AccessImpact
		case "transaction_impact":
			found = hasDomain(facts, DomainPayment) && rePaymentFail.MatchString(text)
		case "customer_impact":
			found = facts.CustomerImpact
		case "outage_evidence":
			found = facts.OutageEvidence
		case "production_context":
			found = facts.ProductionContext
		default:
			found = false
		}

		if found {
			matched = append(matched, req)
		} else {
			if req != "code_block" && req != "diff" && req != "logs" && req != "multi_file_scope" {
				allCritical = false
			}
		}
	}

	if len(matched) == len(required) {
		return matched, "high"
	}
	if len(matched) > 0 && !allCritical {
		return matched, "medium"
	}
	return matched, "low"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
