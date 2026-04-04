package loop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	daedalusgit "github.com/EstebanForge/daedalus/internal/git"
	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/project"
	"github.com/EstebanForge/daedalus/internal/providers"
	"github.com/EstebanForge/daedalus/internal/quality"
)

type fakeProvider struct {
	err        error
	events     []providers.Event
	gotWorkDir *string
	gotRequest *providers.IterationRequest
}

func (p fakeProvider) Name() string {
	return "fake"
}

func (p fakeProvider) Capabilities() providers.Capabilities {
	return providers.Capabilities{}
}

func (p fakeProvider) RunIteration(ctx context.Context, request providers.IterationRequest) (<-chan providers.Event, providers.IterationResult, error) {
	if p.gotWorkDir != nil {
		*p.gotWorkDir = request.WorkDir
	}
	if p.gotRequest != nil {
		copied := request
		copied.ContextFiles = append([]string(nil), request.ContextFiles...)
		*p.gotRequest = copied
	}
	if p.err != nil {
		return nil, providers.IterationResult{}, p.err
	}
	events := make(chan providers.Event, len(p.events))
	for _, event := range p.events {
		events <- event
	}
	close(events)
	return events, providers.IterationResult{Success: true}, nil
}

type fakeChecker struct {
	report quality.Report
	err    error
}

func (c fakeChecker) Run(ctx context.Context, workDir string, commands []string) (quality.Report, error) {
	if c.err != nil {
		return quality.Report{}, c.err
	}
	return c.report, nil
}

type fakeCommitter struct {
	result daedalusgit.CommitResult
	err    error
}

func (c fakeCommitter) CommitStory(ctx context.Context, workDir, storyID, storyTitle string) (daedalusgit.CommitResult, error) {
	if c.err != nil {
		return daedalusgit.CommitResult{}, c.err
	}
	return c.result, nil
}

func TestRunOnceFailsWhenQualityChecksFail(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	manager := NewManager(
		store,
		fakeProvider{},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: false}},
		[]string{"go test ./..."},
		fakeCommitter{},
		CompletionPolicy{},
		nil,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err == nil {
		t.Fatal("expected quality failure error")
	}
}

func TestRunOnceSucceedsWhenQualityChecksPass(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	manager := NewManager(
		store,
		fakeProvider{},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: true, CommitSHA: "abc123"}},
		CompletionPolicy{},
		nil,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	doc, err := store.Load("main")
	if err != nil {
		t.Fatalf("load PRD: %v", err)
	}
	story := doc.UserStories[0]
	if !story.Passes {
		t.Fatal("expected story to be marked as passed")
	}
	if story.InProgress {
		t.Fatal("expected story to be marked as not in progress")
	}
}

func TestRunOnceWritesProviderEventsToArtifacts(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	manager := NewManager(
		store,
		fakeProvider{events: []providers.Event{
			{Type: providers.EventIterationStarted, Message: "started"},
			{Type: providers.EventAssistantText, Message: "working"},
		}},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	eventsData, err := os.ReadFile(project.PRDEventsPath(baseDir, "main"))
	if err != nil {
		t.Fatalf("read events.jsonl: %v", err)
	}
	text := string(eventsData)
	if !strings.Contains(text, "\"type\":\"iteration_started\"") {
		t.Fatalf("expected iteration_started event, got: %s", text)
	}
	if !strings.Contains(text, "\"type\":\"assistant_text\"") {
		t.Fatalf("expected assistant_text event, got: %s", text)
	}

	agentLog, err := os.ReadFile(project.PRDAgentLogPath(baseDir, "main"))
	if err != nil {
		t.Fatalf("read agent.log: %v", err)
	}
	if !strings.Contains(string(agentLog), "[assistant_text] working") {
		t.Fatalf("expected assistant_text log line, got: %s", string(agentLog))
	}
}

func TestRunOnceUsesSeparateArtifactAndExecutionDirs(t *testing.T) {
	t.Parallel()

	artifactDir := t.TempDir()
	execDir := t.TempDir()
	store := prd.NewStore(artifactDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	providerWorkDir := ""
	manager := NewManager(
		store,
		fakeProvider{
			gotWorkDir: &providerWorkDir,
			events: []providers.Event{
				{Type: providers.EventIterationStarted, Message: "started"},
			},
		},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", artifactDir, execDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if providerWorkDir != execDir {
		t.Fatalf("expected provider work dir %q, got %q", execDir, providerWorkDir)
	}
	if _, err := os.Stat(project.PRDEventsPath(artifactDir, "main")); err != nil {
		t.Fatalf("expected events in artifact dir: %v", err)
	}
	if _, err := os.Stat(project.PRDProgressPath(execDir, "main")); !os.IsNotExist(err) {
		t.Fatalf("did not expect progress artifacts in execution dir, err=%v", err)
	}
}

func TestRunOncePersistsQualityCommandDetails(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	manager := NewManager(
		store,
		fakeProvider{},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{
			Passed: true,
			Results: []quality.Result{
				{
					Command:  "go test ./...",
					ExitCode: 0,
					Stdout:   "ok",
					Stderr:   "",
					Duration: 2 * time.Second,
				},
			},
		}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	eventsData, err := os.ReadFile(project.PRDEventsPath(baseDir, "main"))
	if err != nil {
		t.Fatalf("read events.jsonl: %v", err)
	}
	text := string(eventsData)
	if !strings.Contains(text, "\"phase\":\"quality\"") {
		t.Fatalf("expected quality phase event, got: %s", text)
	}
	if !strings.Contains(text, "\"command\":\"go test ./...\"") {
		t.Fatalf("expected quality command in events, got: %s", text)
	}
	if !strings.Contains(text, "\"exitCode\":0") {
		t.Fatalf("expected quality exit code in events, got: %s", text)
	}
	if !strings.Contains(text, "\"stdout\":\"ok\"") {
		t.Fatalf("expected quality stdout in events, got: %s", text)
	}

	progressData, err := os.ReadFile(project.PRDProgressPath(baseDir, "main"))
	if err != nil {
		t.Fatalf("read progress.md: %v", err)
	}
	progressText := string(progressData)
	if !strings.Contains(progressText, "Quality checks:") {
		t.Fatalf("expected quality section in progress, got: %s", progressText)
	}
	if !strings.Contains(progressText, "Command: go test ./...") {
		t.Fatalf("expected command details in progress, got: %s", progressText)
	}
	if !strings.Contains(progressText, "Exit code: 0") {
		t.Fatalf("expected exit code details in progress, got: %s", progressText)
	}
}

func TestRunOnceBuildsPromptAndContextFiles(t *testing.T) {
	t.Parallel()

	artifactDir := t.TempDir()
	execDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(execDir, "AGENTS.md"), []byte("guidance"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(execDir, "README.md"), []byte("readme"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}

	store := prd.NewStore(artifactDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	var gotRequest providers.IterationRequest
	manager := NewManager(
		store,
		fakeProvider{
			gotRequest: &gotRequest,
			events: []providers.Event{
				{Type: providers.EventAssistantText, Message: "done"},
			},
		},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{
			ApprovalPolicy: "never",
			SandboxPolicy:  "workspace-write",
			Model:          "custom-model",
		},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", artifactDir, execDir); err != nil {
		t.Fatalf("run once: %v", err)
	}

	if !strings.Contains(gotRequest.Prompt, "Acceptance Criteria:") {
		t.Fatalf("expected acceptance criteria in prompt, got: %s", gotRequest.Prompt)
	}
	if !strings.Contains(gotRequest.Prompt, "Do not execute destructive git operations.") {
		t.Fatalf("expected safety rule in prompt, got: %s", gotRequest.Prompt)
	}
	if len(gotRequest.ContextFiles) < 5 {
		t.Fatalf("expected required+optional context files, got: %v", gotRequest.ContextFiles)
	}
	if gotRequest.ApprovalPolicy != "never" {
		t.Fatalf("expected approval policy from iteration options, got %q", gotRequest.ApprovalPolicy)
	}
	if gotRequest.Model != "custom-model" {
		t.Fatalf("expected model from iteration options, got %q", gotRequest.Model)
	}
	if gotRequest.SandboxPolicy != "workspace-write" {
		t.Fatalf("expected sandbox policy from iteration options, got %q", gotRequest.SandboxPolicy)
	}
	contextJoined := strings.Join(gotRequest.ContextFiles, "\n")
	if !strings.Contains(contextJoined, ".daedalus/prds/main/prd.md") {
		t.Fatalf("expected prd.md context file, got: %v", gotRequest.ContextFiles)
	}
	if !strings.Contains(contextJoined, "AGENTS.md") {
		t.Fatalf("expected AGENTS.md context file, got: %v", gotRequest.ContextFiles)
	}
	if !strings.Contains(contextJoined, "README.md") {
		t.Fatalf("expected README.md context file, got: %v", gotRequest.ContextFiles)
	}
}

type fakeCompletionExecutor struct {
	pushCalled int
	prCalled   int
	pushErr    error
	prErr      error
}

func (e *fakeCompletionExecutor) PushBranch(_ context.Context, _ string) error {
	e.pushCalled++
	return e.pushErr
}

func (e *fakeCompletionExecutor) CreatePR(_ context.Context, _ string) error {
	e.prCalled++
	return e.prErr
}

func TestRunOnceCallsPushWhenConfigured(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	exec := &fakeCompletionExecutor{}
	manager := NewManager(
		store,
		fakeProvider{},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: true, CommitSHA: "abc123"}},
		CompletionPolicy{PushOnComplete: true},
		exec,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if exec.pushCalled != 1 {
		t.Fatalf("expected push to be called once, got %d", exec.pushCalled)
	}
	if exec.prCalled != 0 {
		t.Fatalf("expected PR creation not to be called, got %d", exec.prCalled)
	}
}

func TestRunOnceCallsPRAfterPush(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	exec := &fakeCompletionExecutor{}
	manager := NewManager(
		store,
		fakeProvider{},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: true, CommitSHA: "abc123"}},
		CompletionPolicy{PushOnComplete: true, AutoPROnComplete: true},
		exec,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if exec.pushCalled != 1 {
		t.Fatalf("expected push to be called once, got %d", exec.pushCalled)
	}
	if exec.prCalled != 1 {
		t.Fatalf("expected PR creation to be called once, got %d", exec.prCalled)
	}
}

func TestRunOnceSkipsPushWhenNoCommit(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	exec := &fakeCompletionExecutor{}
	manager := NewManager(
		store,
		fakeProvider{},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{PushOnComplete: true, AutoPROnComplete: true},
		exec,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if exec.pushCalled != 0 {
		t.Fatalf("expected push not to be called when no commit, got %d", exec.pushCalled)
	}
}

func TestRunOncePushFailureDoesNotFailStory(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	exec := &fakeCompletionExecutor{pushErr: fmt.Errorf("remote unreachable")}
	manager := NewManager(
		store,
		fakeProvider{},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: true, CommitSHA: "abc123"}},
		CompletionPolicy{PushOnComplete: true, AutoPROnComplete: true},
		exec,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("push failure must not fail the story, got: %v", err)
	}

	doc, err := store.Load("main")
	if err != nil {
		t.Fatalf("load PRD: %v", err)
	}
	if !doc.UserStories[0].Passes {
		t.Fatal("expected story to be marked as passed despite push failure")
	}

	agentLog, err := os.ReadFile(project.PRDAgentLogPath(baseDir, "main"))
	if err != nil {
		t.Fatalf("read agent log: %v", err)
	}
	if !strings.Contains(string(agentLog), "[completion] push failed") {
		t.Fatalf("expected push failure in agent log, got: %s", string(agentLog))
	}
}

func TestRunOnceSkipsCompletionWhenPolicyDisabled(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	exec := &fakeCompletionExecutor{}
	manager := NewManager(
		store,
		fakeProvider{},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: true, CommitSHA: "abc123"}},
		CompletionPolicy{},
		exec,
		false,
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if exec.pushCalled != 0 {
		t.Fatalf("expected push not to be called with default policy, got %d", exec.pushCalled)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Compound Engineering Tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPlanPhaseWritesPlanFile(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	planText := "## Implementation Plan\n\nObjective: Do the thing.\n"
	var planRequest providers.IterationRequest
	manager := NewManager(
		store,
		fakeProvider{
			gotRequest: &planRequest,
			events: []providers.Event{
				{Type: providers.EventAssistantText, Message: planText},
			},
		},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		true,  // planEnabled
		nil,   // reviewer
		nil,   // reviewPerspectives
		false, // compoundEnabled
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Check plan file was written
	planPath := project.PRDPlanPath(baseDir, "main", "US-001")
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("expected plan file at %s, got error: %v", planPath, err)
	}
	if !strings.Contains(string(data), "Implementation Plan") {
		t.Fatalf("expected plan content, got: %s", string(data))
	}

	// Check plan file was included in the work request context
	if !strings.Contains(strings.Join(planRequest.ContextFiles, "\n"), "plans/US-001.md") {
		t.Fatalf("expected plan file in work context, got: %v", planRequest.ContextFiles)
	}
}

func TestPlanPhaseSkippedWhenDisabled(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	var workRequest providers.IterationRequest
	manager := NewManager(
		store,
		fakeProvider{
			gotRequest: &workRequest,
			events: []providers.Event{
				{Type: providers.EventAssistantText, Message: "done"},
			},
		},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		false, // planEnabled = false
		nil,
		nil,
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// No plan file should exist
	planPath := project.PRDPlanPath(baseDir, "main", "US-1")
	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Fatalf("expected no plan file, got: %v", err)
	}

	// Work request should not have a plan phase marker
	if strings.Contains(workRequest.Prompt, "Senior engineer creating an implementation plan") {
		t.Fatal("expected no plan prompt in work phase when plan is disabled")
	}
}

func TestCompoundLearningsAppendedOnFailure(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	manager := NewManager(
		store,
		fakeProvider{events: []providers.Event{{Type: providers.EventAssistantText, Message: "done"}}},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: false}},
		[]string{"go test ./..."},
		fakeCommitter{},
		CompletionPolicy{},
		nil,
		false, // planEnabled
		nil,   // reviewer
		nil,   // reviewPerspectives
		true,  // compoundEnabled
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err == nil {
		t.Fatal("expected quality failure error")
	}

	// Check learnings file was created
	learningsPath := project.PRDLearningsPath(baseDir, "main")
	data, err := os.ReadFile(learningsPath)
	if err != nil {
		t.Fatalf("expected learnings file, got error: %v", err)
	}
	if !strings.Contains(string(data), "US-001") {
		t.Fatalf("expected story ID in learnings, got: %s", string(data))
	}
	if !strings.Contains(string(data), "quality") {
		t.Fatalf("expected phase in learnings, got: %s", string(data))
	}
}

func TestCompoundLearningsSkippedWhenDisabled(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	manager := NewManager(
		store,
		fakeProvider{events: []providers.Event{{Type: providers.EventAssistantText, Message: "done"}}},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: false}},
		[]string{"go test ./..."},
		fakeCommitter{},
		CompletionPolicy{},
		nil,
		false, // planEnabled
		nil,   // reviewer
		nil,   // reviewPerspectives
		false, // compoundEnabled = false
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err == nil {
		t.Fatal("expected quality failure error")
	}

	// No learnings file should exist
	learningsPath := project.PRDLearningsPath(baseDir, "main")
	if _, err := os.Stat(learningsPath); !os.IsNotExist(err) {
		t.Fatalf("expected no learnings file when disabled, got: %v", err)
	}
}

func TestCompoundLearningsInjectedIntoContext(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	// Pre-populate learnings file
	learningsPath := project.PRDLearningsPath(baseDir, "main")
	if err := os.WriteFile(learningsPath, []byte("## Previous Learning\n\nAvoid X because Y.\n"), 0o644); err != nil {
		t.Fatalf("write learnings: %v", err)
	}

	var workRequest providers.IterationRequest
	manager := NewManager(
		store,
		fakeProvider{
			gotRequest: &workRequest,
			events: []providers.Event{
				{Type: providers.EventAssistantText, Message: "done"},
			},
		},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		false, // planEnabled
		nil,   // reviewer
		nil,   // reviewPerspectives
		true,  // compoundEnabled
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Check learnings file was included in work context
	contextJoined := strings.Join(workRequest.ContextFiles, "\n")
	if !strings.Contains(contextJoined, "learnings.md") {
		t.Fatalf("expected learnings.md in context, got: %v", workRequest.ContextFiles)
	}
}

type fakeReviewer struct {
	calls int
}

func (r *fakeReviewer) RunReview(_ context.Context, _ string, _ []string, _ []string, _ providers.IterationRequest) (quality.ReviewReport, error) {
	r.calls++
	return quality.ReviewReport{Passed: true}, nil
}

func TestParallelReviewCalledAfterWork(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	reviewer := &fakeReviewer{}
	manager := NewManager(
		store,
		fakeProvider{events: []providers.Event{{Type: providers.EventAssistantText, Message: "done"}}},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		false, // planEnabled
		reviewer,
		[]string{"security", "performance"},
		false, // compoundEnabled
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if reviewer.calls == 0 {
		t.Fatal("expected reviewer to be called at least once")
	}
}

func TestParallelReviewFailureFailsStory(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store := prd.NewStore(baseDir)
	if err := store.Create("main"); err != nil {
		t.Fatalf("create PRD: %v", err)
	}

	badReviewer := &failingReviewer{}
	manager := NewManager(
		store,
		fakeProvider{events: []providers.Event{{Type: providers.EventAssistantText, Message: "done"}}},
		RetryPolicy{MaxRetries: 0, Delays: []time.Duration{0}},
		IterationOptions{},
		fakeChecker{report: quality.Report{Passed: true}},
		[]string{"go test ./..."},
		fakeCommitter{result: daedalusgit.CommitResult{Committed: false}},
		CompletionPolicy{},
		nil,
		false, // planEnabled
		badReviewer,
		[]string{"security"},
		false,
	)

	if err := manager.RunOnce(context.Background(), "main", baseDir, baseDir); err == nil {
		t.Fatal("expected review failure error")
	}
}

type failingReviewer struct{}

func (r *failingReviewer) RunReview(_ context.Context, _ string, _ []string, _ []string, _ providers.IterationRequest) (quality.ReviewReport, error) {
	return quality.ReviewReport{
		Passed: false,
		Reviews: []providers.PerspectiveReview{
			{Perspective: "security", Findings: []string{"hardcoded secret found"}},
		},
	}, nil
}
