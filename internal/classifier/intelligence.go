package classifier

import (
	"dispatch/internal/config"
)

type IntelligenceResult struct {
	Concepts         []string     `json:"concepts,omitempty"`
	OperationObjects []OpObjRole  `json:"operation_objects,omitempty"`
	Similarity       any          `json:"similarity,omitempty"`
	Session          *SessionInfo `json:"session,omitempty"`
	Profile          *ProfileInfo `json:"profile,omitempty"`
	ExemplarLevel    string       `json:"-"`
	ExemplarSource   string       `json:"-"`
	SessionEscalated bool         `json:"-"`
	EscalationLevel  string       `json:"-"`
	ProfileInfo      *ProfileInfo `json:"-"`
	ExemplarsUsed    int          `json:"exemplars_used,omitempty"`
	MergeMode        string       `json:"exemplars_merge_mode,omitempty"`
}

func (r *IntelligenceResult) ToMap() map[string]any {
	if r == nil {
		return nil
	}
	m := make(map[string]any)

	if len(r.Concepts) > 0 {
		m["concepts"] = r.Concepts
	}
	if len(r.OperationObjects) > 0 {
		m["operation_objects"] = r.OperationObjects
	}
	if r.Similarity != nil {
		m["similarity"] = r.Similarity
	}
	if r.Session != nil {
		m["session"] = r.Session
	}
	if r.Profile != nil {
		m["profile"] = r.Profile
	}
	if r.ExemplarsUsed > 0 {
		m["exemplars_used"] = r.ExemplarsUsed
		m["exemplars_merge_mode"] = r.MergeMode
	}

	return m
}

func evaluateIntelligence(facts Facts, text string, cfg *config.Config, sessionID, taskKey string) *IntelligenceResult {
	if cfg.Intelligence == nil || !cfg.Intelligence.Enabled {
		return nil
	}

	result := &IntelligenceResult{}

	concepts := ExtractConcepts(text, cfg)
	if len(concepts) > 0 {
		result.Concepts = concepts
	}

	opObjs := ExtractOperationObjectRoles(facts, text, cfg)
	if len(opObjs) > 0 {
		result.OperationObjects = opObjs
	}

	simResult := ComputeSimilarity(text, facts)
	if simResult != nil {
		simMap := map[string]any{
			"mode": simResult.Method,
			"resolver": map[string]any{
				"exemplar_level": simResult.Level,
				"confidence":     simResult.Confidence,
				"source":         simResult.Source,
				"agreement":      simResult.Agreement,
			},
		}
		result.Similarity = simMap
		result.ExemplarsUsed = len(exemplars)

		if simResult.Level != "" && cfg.Intelligence != nil && cfg.Intelligence.Uncertainty.ExemplarFloorWeight > 0 {
			if simResult.Confidence >= cfg.Intelligence.Uncertainty.ExemplarFloorWeight {
				exemplarLevel := simResult.Level
				if levelRank(exemplarLevel) > levelRank("hard") {
					exemplarLevel = "hard"
				}
				result.ExemplarLevel = exemplarLevel
				result.ExemplarSource = simResult.Source
			}
		}
	}

	sessLevel, sessInfo, _ := CheckSessionEscalation(sessionID, taskKey, facts, cfg)
	if sessInfo != nil && sessInfo.Tracked {
		result.Session = sessInfo
		if sessInfo.Escalated && sessLevel != "" {
			result.SessionEscalated = true
			result.EscalationLevel = sessLevel
		}
	}

	UpdateSession(sessionID, taskKey, facts, cfg)

	return result
}
