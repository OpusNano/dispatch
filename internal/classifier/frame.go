package classifier

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"

	"dispatch/internal/config"
)

type TaskFrame struct {
	LatestUserIndex   int `json:"latest_user_index"`
	TaskBoundaryIndex int `json:"task_boundary_index"`

	ContinuationDetected bool     `json:"continuation_detected"`
	ContinuationReasons  []string `json:"continuation_reasons,omitempty"`

	TaskKey       string `json:"task_key"`
	TaskKeySource string `json:"task_key_source"`

	FrameText            string `json:"-"`
	ToolResultCount      int    `json:"-"`
	ExcludedCount        int    `json:"-"`
	FailureEvidenceCount int    `json:"-"`

	ExcludedHardContext      bool   `json:"excluded_prior_hard_context"`
	PriorLevelIgnored        bool   `json:"prior_level_ignored_for_routing"`
	SessionUsedPreviousState bool   `json:"session_used_previous_state"`
	SessionUsedCurrentFrame  bool   `json:"session_used_current_frame"`
	SessionEscalationReason  string `json:"session_escalation_reason,omitempty"`
}

var (
	reSameError       = regexp.MustCompile(`(?i)\b(same error|that error|this error|same (issue|bug|problem)|that (issue|bug|problem)|this (issue|bug|problem))\b`)
	reStillFailing    = regexp.MustCompile(`(?i)\b(still (failing|broken|not working|crashing|happens|present))\b`)
	reExplicitCont    = regexp.MustCompile(`(?i)\b(continue from previous|continue with that|carry on|pick up where|resume (from |the )?previous)\b`)
	reTryRunAgain     = regexp.MustCompile(`(?i)^\s*(try again|run (it |that |the (tests?|test) )?again|retry|do it again)\b`)
	reContPatchFailed = regexp.MustCompile(`(?i)\b(patch (did not|failed to|does not|won't) apply|patch (rejected|error))\b`)
	reTestsStillFail  = regexp.MustCompile(`(?i)\b(tests? (are |still )?fail(ing|ed|ure))\b`)
	reCompileStillErr = regexp.MustCompile(`(?i)\b(compil(e|ation|ing) (still )?(error|fail|broken))\b`)
	rePrevTaskRef     = regexp.MustCompile(`(?i)\b((the |that |my )?previous (task|fix|patch|change|attempt|step|approach|solution)|what (I |you |we )?(did|tried|had) (before|earlier|last time))\b`)
)

type continuationDetector struct {
	re    *regexp.Regexp
	label string
}

var continuationDetectors = []continuationDetector{
	{reTryRunAgain, "try_again"},
	{reSameError, "same_error"},
	{reStillFailing, "still_failing"},
	{reExplicitCont, "explicit_continue"},
	{reContPatchFailed, "patch_failed"},
	{reTestsStillFail, "tests_still_fail"},
	{reCompileStillErr, "compile_still_fail"},
	{rePrevTaskRef, "previous_task_ref"},
}

func ExtractTaskFrame(messages []Message, taskIDHeader string, _ *config.Config) TaskFrame {
	frame := TaskFrame{
		PriorLevelIgnored: true,
	}

	latestUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			latestUserIdx = i
			break
		}
	}
	if latestUserIdx < 0 {
		frame.LatestUserIndex = 0
		frame.TaskBoundaryIndex = 0
		frame.FrameText = concatMessages(messages)
		frame.ToolResultCount = countToolMessages(messages, 0)
		frame.TaskKey, frame.TaskKeySource = deriveTaskKey(taskIDHeader, "")
		return frame
	}

	frame.LatestUserIndex = latestUserIdx
	frame.TaskBoundaryIndex = latestUserIdx

	priorUserIdx := -1
	for i := latestUserIdx - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			priorUserIdx = i
			break
		}
	}

	if priorUserIdx >= 0 {
		latestText := messages[latestUserIdx].Content
		if cont, reasons := detectContinuation(latestText, messages[priorUserIdx].Content); cont {
			frame.ContinuationDetected = true
			frame.ContinuationReasons = reasons
			frame.TaskBoundaryIndex = priorUserIdx
			frame.PriorLevelIgnored = false

			for {
				prevUser := findPriorUser(messages, frame.TaskBoundaryIndex)
				if prevUser < 0 {
					break
				}
				if cont2, _ := detectContinuation(messages[frame.TaskBoundaryIndex].Content, messages[prevUser].Content); cont2 {
					frame.TaskBoundaryIndex = prevUser
				} else {
					break
				}
			}
		}
	}

	frame.FrameText = concatMessages(messages[frame.TaskBoundaryIndex:])
	frame.ToolResultCount = countToolMessages(messages, frame.TaskBoundaryIndex)
	frame.ExcludedCount = frame.TaskBoundaryIndex

	if frame.TaskBoundaryIndex > 0 {
		excludedText := concatMessages(messages[0:frame.TaskBoundaryIndex])
		frame.ExcludedHardContext = textHasHardContext(excludedText)
	} else {
		frame.ExcludedHardContext = false
	}

	boundaryText := messages[frame.TaskBoundaryIndex].Content
	frame.TaskKey, frame.TaskKeySource = deriveTaskKey(taskIDHeader, boundaryText)

	return frame
}

func detectContinuation(latestText, priorText string) (bool, []string) {
	_ = priorText
	var reasons []string
	for _, d := range continuationDetectors {
		if d.re.MatchString(latestText) {
			reasons = append(reasons, d.label)
		}
	}
	if len(reasons) > 0 {
		return true, reasons
	}
	return false, nil
}

func findPriorUser(messages []Message, currentIdx int) int {
	for i := currentIdx - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return i
		}
	}
	return -1
}

func countToolMessages(messages []Message, startIdx int) int {
	count := 0
	for i := startIdx; i < len(messages); i++ {
		if messages[i].Role == "tool" {
			count++
		}
	}
	return count
}

func textHasHardContext(text string) bool {
	return reStackTrace.MatchString(text) ||
		reTestFailure.MatchString(text) ||
		reCompileError.MatchString(text) ||
		reToolError.MatchString(text) ||
		reProduction.MatchString(text) ||
		reSecurityDomain.MatchString(text) ||
		reSecretLeak.MatchString(text) ||
		reDataLoss.MatchString(text)
}

func deriveTaskKey(taskIDHeader, boundaryText string) (string, string) {
	if taskIDHeader != "" {
		return taskIDHeader, "header"
	}
	hash := hashUserTurn(boundaryText)
	return hash, "derived_from_boundary_user"
}

func hashUserTurn(text string) string {
	normalized := normalizeText(text)
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:8])
}

func normalizeText(text string) string {
	if len(text) > 512 {
		text = text[:512]
	}
	text = strings.ToLower(text)
	var buf strings.Builder
	buf.Grow(len(text))
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			buf.WriteRune(r)
		} else {
			buf.WriteRune(' ')
		}
	}
	collapsed := strings.Join(strings.Fields(buf.String()), " ")
	return collapsed
}
