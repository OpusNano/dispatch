package classifier

import (
	"fmt"

	"dispatch/internal/config"
)

type Classification struct {
	Level        string      `json:"level"`
	Model        string      `json:"model"`
	Scores       Scores      `json:"scores"`
	Reasons      []string    `json:"reasons"`
	ForcedBy     string      `json:"forced_by,omitempty"`
	Analysis     Analysis    `json:"analysis"`
	Intelligence any         `json:"intelligence,omitempty"`
	Context      ContextMeta `json:"-"`
	Frame        *TaskFrame  `json:"frame,omitempty"`
}

type Scores struct {
	Complexity    float64 `json:"complexity"`
	Risk          float64 `json:"risk"`
	AgentPressure float64 `json:"agent_pressure"`
	Downgrade     float64 `json:"downgrade"`
	Total         float64 `json:"total"`
}

type Input struct {
	Messages          []Message `json:"messages"`
	HasTools          bool      `json:"has_tools"`
	HasResponseFormat bool      `json:"has_response_format"`
}

type Message struct {
	Role       string `json:"role"`
	Content    string `json:"content,omitempty"`
	ToolCalls  []any  `json:"tool_calls,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

func Classify(input Input, cfg *config.Config, sessionID, taskKey, profileName string) Classification {
	combinedText := concatMessages(input.Messages)
	contextMeta := ComputeContextMetadata(combinedText, len(input.Messages))

	facts := ExtractFacts(combinedText, input.HasTools, input.HasResponseFormat)

	scores, scoreReasons := computeScores(input, cfg, combinedText)

	scoreLevel := mapTotalToLevel(scores.Total, cfg.Thresholds)

	if scores.Risk >= cfg.Thresholds.RiskCriticalOverride {
		scoreLevel = "critical"
		scoreReasons = append(scoreReasons, fmt.Sprintf("override: risk %.1f >= %.1f -> critical", scores.Risk, cfg.Thresholds.RiskCriticalOverride))
	}
	if scores.AgentPressure >= cfg.Thresholds.AgentPressureCriticalOverride {
		scoreLevel = "critical"
		scoreReasons = append(scoreReasons, fmt.Sprintf("override: agent_pressure %.1f >= %.1f -> critical", scores.AgentPressure, cfg.Thresholds.AgentPressureCriticalOverride))
	}
	if scoreLevel != "critical" && scoreLevel != "hard" && scores.Risk >= cfg.Thresholds.RiskHardFloor {
		scoreLevel = "hard"
		scoreReasons = append(scoreReasons, fmt.Sprintf("override: risk %.1f >= %.1f -> floor hard", scores.Risk, cfg.Thresholds.RiskHardFloor))
	}

	level, analysis, policyReasons := EvaluatePolicy(facts, scoreLevel, combinedText)

	gatesFired := len(analysis.CriticalGates) > 0
	evidenceFloorsFired := len(analysis.Floors) > 0

	intelResult := evaluateIntelligence(facts, combinedText, cfg, sessionID, taskKey)
	if intelResult != nil {
		if intelResult.ExemplarLevel != "" && levelRank(intelResult.ExemplarLevel) > levelRank(level) {
			level = intelResult.ExemplarLevel
			reasons := append(policyReasons, "exemplar:"+intelResult.ExemplarSource+" level="+intelResult.ExemplarLevel)
			policyReasons = reasons
		}
		if intelResult.SessionEscalated && intelResult.EscalationLevel != "" {
			level = maxLevel(level, intelResult.EscalationLevel)
		}
		if intelResult.ProfileInfo != nil || profileName != "" {
			profLevel, profInfo := ApplyProfile(level, facts, gatesFired, profileName, cfg)
			if profInfo != nil {
				intelResult.ProfileInfo = profInfo
			}
			if profLevel != "" {
				level = profLevel
			}
		}
	}

	level = applyLengthCap(level, evidenceFloorsFired, gatesFired || (intelResult != nil && intelResult.SessionEscalated))
	if levelRank(level) > levelRank(scoreLevel) && !evidenceFloorsFired && !gatesFired && (intelResult == nil || !intelResult.SessionEscalated) {
		analysis.LengthPolicy = "capped_at_medium_without_evidence"
	}

	reasons := append(policyReasons, scoreReasons...)

	if level == "medium" && levelRank(scoreLevel) > levelRank("medium") && !evidenceFloorsFired {
		reasons = append(reasons, "length_policy:capped_at_medium_without_evidence")
	}

	analysis.Context = contextMeta
	populateAnalysisEvidence(&analysis, facts)

	levelCfg := cfg.Levels[level]

	cls := Classification{
		Level:    level,
		Model:    levelCfg.Model,
		Scores:   scores,
		Reasons:  reasons,
		Analysis: analysis,
		Context:  contextMeta,
	}

	if intelResult != nil {
		cls.Intelligence = intelResult.ToMap()
	}

	return cls
}

func ClassifySimple(input Input, cfg *config.Config) Classification {
	return Classify(input, cfg, "", "", "")
}

func populateAnalysisEvidence(analysis *Analysis, facts Facts) {
	if facts.DestructiveAction || facts.DataLossEvidence {
		analysis.Irreversibility = "present"
	} else {
		analysis.Irreversibility = "none"
	}

	var failures []string
	if hasEvidence(facts.Evidence, EvidenceStackTrace) {
		failures = append(failures, "stack_trace")
	}
	if hasEvidence(facts.Evidence, EvidenceTestFailure) {
		failures = append(failures, "test_failure")
	}
	if hasEvidence(facts.Evidence, EvidenceCompileError) {
		failures = append(failures, "compile_error")
	}
	if hasEvidence(facts.Evidence, EvidenceToolError) {
		failures = append(failures, "tool_error")
	}
	if hasEvidence(facts.Evidence, EvidenceRepeatedFailure) {
		failures = append(failures, "repeated_failure")
	}
	analysis.FailureEvidence = failures

	hasStructural := hasEvidence(facts.Evidence, EvidenceCodeBlock) ||
		hasEvidence(facts.Evidence, EvidenceDiff) ||
		hasEvidence(facts.Evidence, EvidenceJSONSchema) ||
		facts.Scope == ScopeMultiFile
	if hasStructural {
		analysis.StructuralComplexity = "medium"
	} else if hasEvidence(facts.Evidence, EvidenceToolCalls) {
		analysis.StructuralComplexity = "low"
	} else {
		analysis.StructuralComplexity = "low"
	}

	hasRiskyTopics := len(facts.Domains) > 0
	var hasEvidenceSignal bool
	for _, d := range facts.Domains {
		if d == DomainUnknown || d == DomainCode || d == DomainDocs || d == DomainUI {
			continue
		}
		hasRiskyTopics = true
	}
	if hasRiskyTopics && len(analysis.Floors) == 0 && len(analysis.CriticalGates) == 0 {
		analysis.TopicEscalation = "ignored_by_policy"
	}
	if !hasEvidenceSignal {
		_ = hasEvidenceSignal
	}

	var topics []string
	for _, d := range facts.Domains {
		if d != DomainUnknown {
			topics = append(topics, string(d))
		}
	}
	analysis.Topics = topics
}

func computeScores(input Input, cfg *config.Config, combinedText string) (Scores, []string) {
	scores := Scores{}
	var reasons []string

	patterns := cfg.CompiledPatternsOrdered()
	firedPatterns := map[string]bool{}

	for _, cr := range patterns {
		p := cr.Pattern
		var matched bool
		totalWeight := 0.0

		if isCombinationRule(p) {
			if allFired(p.Requires, firedPatterns) && noneFired(p.RequiresNot, firedPatterns) {
				matched = true
				totalWeight = p.Weight
			}
		} else if p.MatchTools {
			matched = input.HasTools
			if matched {
				totalWeight = p.Weight
			}
		} else if p.MatchResponseFormat {
			matched = input.HasResponseFormat
			if matched {
				totalWeight = p.Weight
			}
		} else if cr.Regex != nil {
			if p.PerMatch {
				matches := cr.FindAllMatches(combinedText)
				if len(matches) > 0 {
					matched = true
					rawTotal := float64(len(matches)) * p.Weight
					if p.Cap > 0 && rawTotal > p.Cap {
						rawTotal = p.Cap
					}
					totalWeight = rawTotal
				}
			} else {
				if cr.MatchesText(combinedText) {
					matched = true
					totalWeight = p.Weight
				}
			}
		} else {
			continue
		}

		if matched {
			if !noneFired(p.RequiresNot, firedPatterns) {
				continue
			}

			firedPatterns[p.ID] = true

			reasonText := p.Reason
			if p.PerMatch && cr.Regex != nil {
				count := len(cr.FindAllMatches(combinedText))
				reasonText = fmt.Sprintf("%s (%d matches, score %.1f)", p.Reason, count, totalWeight)
			}

			switch p.Dimension {
			case "complexity":
				scores.Complexity += totalWeight
				reasons = append(reasons, fmt.Sprintf("+complexity: %s", reasonText))
			case "risk":
				scores.Risk += totalWeight
				reasons = append(reasons, fmt.Sprintf("+risk: %s", reasonText))
			case "agent_pressure":
				scores.AgentPressure += totalWeight
				reasons = append(reasons, fmt.Sprintf("+agent_pressure: %s", reasonText))
			case "downgrade":
				scores.Downgrade += totalWeight
				reasons = append(reasons, fmt.Sprintf("+downgrade: %s", reasonText))
			}
		}
	}

	sc := cfg.Scoring
	if scores.Complexity > sc.ComplexityCap {
		scores.Complexity = sc.ComplexityCap
	}
	if scores.Risk > sc.RiskCap {
		scores.Risk = sc.RiskCap
	}
	if scores.AgentPressure > sc.AgentPressureCap {
		scores.AgentPressure = sc.AgentPressureCap
	}
	if scores.Downgrade > sc.DowngradeCap {
		scores.Downgrade = sc.DowngradeCap
	}

	w := sc.Weights
	scores.Total = w.Complexity*scores.Complexity +
		w.Risk*scores.Risk +
		w.AgentPressure*scores.AgentPressure +
		w.Downgrade*scores.Downgrade

	return scores, reasons
}

func mapTotalToLevel(total float64, th config.ThresholdsConfig) string {
	switch {
	case total <= th.EasyMax:
		return "easy"
	case total <= th.MediumMax:
		return "medium"
	case total <= th.HardMax:
		return "hard"
	default:
		return "critical"
	}
}

func concatMessages(msgs []Message) string {
	var result string
	for _, msg := range msgs {
		result += msg.Content
	}
	return result
}

func isCombinationRule(p config.PatternRule) bool {
	return len(p.Requires) > 0
}

func allFired(required []string, fired map[string]bool) bool {
	for _, id := range required {
		if !fired[id] {
			return false
		}
	}
	return true
}

func noneFired(excluded []string, fired map[string]bool) bool {
	for _, id := range excluded {
		if fired[id] {
			return false
		}
	}
	return true
}
