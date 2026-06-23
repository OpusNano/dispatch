package classifier

import (
	"testing"

	"dispatch/internal/config"
)

func msg(role, content string) Message {
	return Message{Role: role, Content: content}
}

func TestNewStandaloneTurn(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "let me look at that"),
		msg("tool", "stack trace: panic at auth.go:42"),
		msg("tool", "compile error: undefined reference to foo"),
		msg("user", "what is a closure?"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.LatestUserIndex != 4 {
		t.Errorf("expected LatestUserIndex=4, got %d", frame.LatestUserIndex)
	}
	if frame.TaskBoundaryIndex != 4 {
		t.Errorf("expected TaskBoundaryIndex=4, got %d", frame.TaskBoundaryIndex)
	}
	if frame.ContinuationDetected {
		t.Error("expected no continuation")
	}
	if frame.ExcludedCount != 4 {
		t.Errorf("expected ExcludedCount=4, got %d", frame.ExcludedCount)
	}
	if !frame.ExcludedHardContext {
		t.Error("expected ExcludedHardContext=true (stack trace in old msgs)")
	}
	if frame.PriorLevelIgnored != true {
		t.Error("expected PriorLevelIgnored=true for new standalone turn")
	}
	if frame.ToolResultCount != 0 {
		t.Errorf("expected ToolResultCount=0, got %d", frame.ToolResultCount)
	}
}

func TestContinuationSameError(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("tool", "error: something went wrong"),
		msg("user", "same error still happens"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("expected continuation")
	}
	if frame.TaskBoundaryIndex != 0 {
		t.Errorf("expected TaskBoundaryIndex=0, got %d", frame.TaskBoundaryIndex)
	}
	foundSameError := false
	foundStillFailing := false
	for _, r := range frame.ContinuationReasons {
		if r == "same_error" {
			foundSameError = true
		}
		if r == "still_failing" {
			foundStillFailing = true
		}
	}
	if !foundSameError {
		t.Error("expected same_error in continuation reasons")
	}
	if !foundStillFailing {
		t.Error("expected still_failing in continuation reasons")
	}
}

func TestContinuationExplicitContinue(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "continue from previous"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("expected continuation for 'continue from previous'")
	}
	if frame.TaskBoundaryIndex != 0 {
		t.Errorf("expected TaskBoundaryIndex=0, got %d", frame.TaskBoundaryIndex)
	}
}

func TestContinuationTryAgain(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "try again"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("expected continuation for 'try again'")
	}
	if frame.TaskBoundaryIndex != 0 {
		t.Errorf("expected TaskBoundaryIndex=0, got %d", frame.TaskBoundaryIndex)
	}
}

func TestContinuationRunItAgain(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "run it again"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("expected continuation for 'run it again'")
	}
}

func TestContinuationTestsStillFailing(t *testing.T) {
	messages := []Message{
		msg("user", "fix failing tests"),
		msg("assistant", "ok"),
		msg("user", "tests still failing"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("expected continuation for 'tests still failing'")
	}
	if frame.TaskBoundaryIndex != 0 {
		t.Errorf("expected TaskBoundaryIndex=0, got %d", frame.TaskBoundaryIndex)
	}
}

func TestContinuationPatchFailed(t *testing.T) {
	messages := []Message{
		msg("user", "apply this patch"),
		msg("assistant", "ok"),
		msg("user", "patch did not apply"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("expected continuation for 'patch did not apply'")
	}
}

func TestContinuationPreviousPatch(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "the previous patch from earlier"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("expected continuation for 'previous patch'")
	}
}

func TestBareYesResets(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("tool", "stack trace: panic at auth.go:42"),
		msg("tool", "compile error: undefined reference"),
		msg("user", "yes"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("bare 'yes' must NOT force continuation")
	}
	if frame.TaskBoundaryIndex != 4 {
		t.Errorf("expected TaskBoundaryIndex=4 (latest user), got %d", frame.TaskBoundaryIndex)
	}
	if !frame.ExcludedHardContext {
		t.Error("expected ExcludedHardContext=true")
	}
}

func TestBareWhyResets(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("tool", "stack trace error"),
		msg("user", "why?"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("bare 'why?' must NOT force continuation")
	}
	if frame.TaskBoundaryIndex != 3 {
		t.Errorf("expected TaskBoundaryIndex=3, got %d", frame.TaskBoundaryIndex)
	}
}

func TestBareOkResets(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "ok"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("bare 'ok' must NOT force continuation")
	}
}

func TestBareSureResets(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "sure"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("bare 'sure' must NOT force continuation")
	}
}

func TestBareGotItResets(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "got it"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("bare 'got it' must NOT force continuation")
	}
}

func TestBareThanksResets(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "thanks"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("bare 'thanks' must NOT force continuation")
	}
}

func TestNoNewUserAfterTools(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "let me check"),
		msg("tool", "stack trace: panic at auth.go:42"),
		msg("tool", "test failure: 3 tests failed"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.LatestUserIndex != 0 {
		t.Errorf("expected LatestUserIndex=0, got %d", frame.LatestUserIndex)
	}
	if frame.TaskBoundaryIndex != 0 {
		t.Errorf("expected TaskBoundaryIndex=0, got %d", frame.TaskBoundaryIndex)
	}
	if frame.ContinuationDetected {
		t.Error("single user turn should not detect continuation")
	}
	if frame.ToolResultCount != 2 {
		t.Errorf("expected ToolResultCount=2, got %d", frame.ToolResultCount)
	}
	if frame.ExcludedCount != 0 {
		t.Errorf("expected ExcludedCount=0, got %d", frame.ExcludedCount)
	}
}

func TestUnrelatedPrefixResets(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("tool", "error stack trace"),
		msg("user", "unrelated: how do I list files?"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("expected no continuation for clearly unrelated question")
	}
	if frame.TaskBoundaryIndex != 3 {
		t.Errorf("expected TaskBoundaryIndex=3, got %d", frame.TaskBoundaryIndex)
	}
}

func TestStandaloneKnowledgeQuestion(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug in production, there's a stack trace showing panic"),
		msg("assistant", "ok"),
		msg("tool", "goroutine 1 [running]: main.main() /tmp/main.go:10 +0x45"),
		msg("tool", "3 tests failed FAIL"),
		msg("assistant", "here is the fix"),
		msg("user", "what is a closure?"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("learning question should NOT continue debugging task")
	}
	if frame.TaskBoundaryIndex != 5 {
		t.Errorf("expected TaskBoundaryIndex=5, got %d", frame.TaskBoundaryIndex)
	}
	if !frame.ExcludedHardContext {
		t.Error("expected ExcludedHardContext=true")
	}
}

func TestContinuationDerivesTaskKeyFromBoundary(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug in production"),
		msg("assistant", "ok"),
		msg("tool", "error stack trace"),
		msg("user", "same error still happens"),
	}
	frame1 := ExtractTaskFrame(messages, "", &config.Config{})

	if frame1.TaskKeySource != "derived_from_boundary_user" {
		t.Errorf("expected derived_from_boundary_user, got %s", frame1.TaskKeySource)
	}

	messages2 := []Message{
		msg("user", "fix auth bug in production"),
		msg("assistant", "ok"),
		msg("tool", "error stack trace"),
		msg("user", "same error still happens"),
		msg("assistant", "ok"),
		msg("user", "same error still happens again"),
	}
	frame2 := ExtractTaskFrame(messages2, "", &config.Config{})

	if frame1.TaskKey != frame2.TaskKey {
		t.Errorf("continuation should derive same task key from original boundary user: %s != %s", frame1.TaskKey, frame2.TaskKey)
	}
}

func TestNewTaskDerivesNewTaskKey(t *testing.T) {
	messagesA := []Message{
		msg("user", "fix auth bug in production"),
	}
	frameA := ExtractTaskFrame(messagesA, "", &config.Config{})

	messagesB := []Message{
		msg("user", "explain closures"),
	}
	frameB := ExtractTaskFrame(messagesB, "", &config.Config{})

	if frameA.TaskKey == frameB.TaskKey {
		t.Error("different task texts should produce different task keys")
	}
}

func TestLengthContaminationPrevented(t *testing.T) {
	bigText := ""
	for i := 0; i < 1000; i++ {
		bigText += "This is old hard context with stack traces and test failures.\n" +
			"panic: runtime error at line 42.\n" +
			"FAIL 3 tests failed out of 10.\n"
	}

	messages := []Message{
		msg("user", bigText),
		msg("assistant", "ok"),
		msg("tool", "more error output"),
		msg("user", "explain briefly"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("new task should not continue")
	}
	if len(frame.FrameText) > len("explain briefly")+10 {
		t.Errorf("frame text should be short, got %d chars", len(frame.FrameText))
	}
	if frame.ExcludedCount != 3 {
		t.Errorf("expected ExcludedCount=3, got %d", frame.ExcludedCount)
	}
}

func TestTaskIDHeaderUsed(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
	}
	frame := ExtractTaskFrame(messages, "custom-task-id-123", &config.Config{})

	if frame.TaskKey != "custom-task-id-123" {
		t.Errorf("expected custom-task-id-123, got %s", frame.TaskKey)
	}
	if frame.TaskKeySource != "header" {
		t.Errorf("expected source=header, got %s", frame.TaskKeySource)
	}
}

func TestBareFileNameDoesNotContinue(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug in auth.go"),
		msg("assistant", "ok"),
		msg("user", "auth.go needs changes"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("bare file name should NOT force continuation")
	}
}

func TestBareFunctionNameDoesNotContinue(t *testing.T) {
	messages := []Message{
		msg("user", "fix ValidateToken function"),
		msg("assistant", "ok"),
		msg("user", "fix ValidateToken"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("bare function name should NOT force continuation")
	}
}

func TestSameFileStillFailingContinues(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "still failing in auth.go"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("still failing in file should continue")
	}
}

func TestSameFileSameErrorContinues(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "ok"),
		msg("user", "same error in auth.go"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("same error in file should continue")
	}
}

func TestContinuationCompileStillFailing(t *testing.T) {
	messages := []Message{
		msg("user", "fix the build"),
		msg("assistant", "ok"),
		msg("user", "compilation still failing"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if !frame.ContinuationDetected {
		t.Error("compilation still failing should continue")
	}
}

func TestOldToolOutputExcludedFromNewTask(t *testing.T) {
	messages := []Message{
		msg("user", "fix auth bug"),
		msg("assistant", "let me check"),
		msg("tool", "stack trace: panic at auth.go:42"),
		msg("tool", "test failure: 3 tests failed"),
		msg("user", "explain closures"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.ContinuationDetected {
		t.Error("new task should not continue")
	}
	if frame.ToolResultCount != 0 {
		t.Errorf("expected ToolResultCount=0 (tool msgs excluded), got %d", frame.ToolResultCount)
	}
	if frame.TaskBoundaryIndex != 4 {
		t.Errorf("expected TaskBoundaryIndex=4 (new user), got %d", frame.TaskBoundaryIndex)
	}
}

func TestContinuationPreviousTaskRef(t *testing.T) {
	tests := []string{
		"same issue as before in auth.go",
		"previous task still has the same bug",
		"what I tried earlier didn't work",
		"my previous fix broke the build",
	}
	for _, text := range tests {
		messages := []Message{
			msg("user", "fix auth bug"),
			msg("assistant", "ok"),
			msg("user", text),
		}
		frame := ExtractTaskFrame(messages, "", &config.Config{})
		if !frame.ContinuationDetected {
			t.Errorf("expected continuation for text: %q", text)
		}
	}
}

func TestHashUserTurnConsistent(t *testing.T) {
	a := hashUserTurn("fix auth bug in Production!")
	b := hashUserTurn("Fix auth bug  in  production")
	c := hashUserTurn("  FIX AUTH BUG IN PRODUCTION  ")
	if a != b || b != c {
		t.Errorf("hashes should be consistent: %s, %s, %s", a, b, c)
	}

	different := hashUserTurn("explain closures")
	if a == different {
		t.Error("different texts should have different hashes")
	}
}

func TestOnlySystemNoUser(t *testing.T) {
	messages := []Message{
		msg("system", "you are a helpful assistant"),
		msg("assistant", "hello"),
	}
	frame := ExtractTaskFrame(messages, "", &config.Config{})

	if frame.LatestUserIndex != 0 {
		t.Errorf("expected LatestUserIndex=0 when no user, got %d", frame.LatestUserIndex)
	}
	if frame.TaskBoundaryIndex != 0 {
		t.Errorf("expected TaskBoundaryIndex=0 when no user, got %d", frame.TaskBoundaryIndex)
	}
}
