package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	daedalusgit "github.com/EstebanForge/daedalus/internal/git"
	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/project"
	"github.com/EstebanForge/daedalus/internal/providers"
	"github.com/EstebanForge/daedalus/internal/quality"
)

type RetryPolicy struct {
	MaxRetries int
	Delays     []time.Duration
}

type IterationOptions struct {
	ApprovalPolicy string
	SandboxPolicy  string
	Model          string
}

type CompletionPolicy struct {
	PushOnComplete   bool
	AutoPROnComplete bool
}

type qualityChecker interface {
	Run(ctx context.Context, workDir string, commands []string) (quality.Report, error)
}

type committer interface {
	CommitStory(ctx context.Context, workDir, storyID, storyTitle string) (daedalusgit.CommitResult, error)
}

type completionExecutor interface {
	PushBranch(ctx context.Context, workDir string) error
	CreatePR(ctx context.Context, workDir string) error
}

type Manager struct {
	store              prd.Store
	provider           providers.Provider
	retry              RetryPolicy
	iteration          IterationOptions
	qualityChecker     qualityChecker
	qualityCommands    []string
	committer          committer
	completion         CompletionPolicy
	completionExec     completionExecutor
	planEnabled        bool
	reviewer           quality.Reviewer
	reviewPerspectives []string
	compoundEnabled    bool
}

func NewManager(
	store prd.Store,
	provider providers.Provider,
	retry RetryPolicy,
	iteration IterationOptions,
	checker qualityChecker,
	qualityCommands []string,
	commitService committer,
	completionPolicy CompletionPolicy,
	completionExec completionExecutor,
	planEnabled bool,
	reviewer quality.Reviewer,
	reviewPerspectives []string,
	compoundEnabled bool,
) Manager {
	if strings.TrimSpace(iteration.ApprovalPolicy) == "" {
		iteration.ApprovalPolicy = "on-failure"
	}
	if strings.TrimSpace(iteration.SandboxPolicy) == "" {
		iteration.SandboxPolicy = "workspace-write"
	}
	if strings.TrimSpace(iteration.Model) == "" {
		iteration.Model = "default"
	}

	return Manager{
		store:              store,
		provider:           provider,
		retry:              retry,
		iteration:          iteration,
		qualityChecker:     checker,
		qualityCommands:    qualityCommands,
		committer:          commitService,
		completion:         completionPolicy,
		completionExec:     completionExec,
		planEnabled:        planEnabled,
		reviewer:           reviewer,
		reviewPerspectives: reviewPerspectives,
		compoundEnabled:    compoundEnabled,
	}
}

func (m Manager) RunOnce(ctx context.Context, name string, artifactDir string, workDir string) error {
	if strings.TrimSpace(artifactDir) == "" {
		artifactDir = workDir
	}

	doc, err := m.store.Load(name)
	if err != nil {
		return err
	}

	story := doc.NextStory()
	if story == nil {
		return nil
	}
	storyID := story.ID
	storyTitle := story.Title

	if !story.InProgress {
		if err := setStoryInProgress(&doc, storyID); err != nil {
			return err
		}
		if err := m.store.Save(name, doc); err != nil {
			return err
		}
	}

	// Build base context, optionally injecting learnings.
	contextFiles := m.buildContextFiles(artifactDir, workDir, name)
	if m.compoundEnabled {
		contextFiles = m.injectLearnings(contextFiles, artifactDir, name)
	}

	// ── PHASE 1: Plan (optional) ──────────────────────────────────────────────
	var planPath string
	if m.planEnabled {
		planPath, err = m.runPlanPhase(ctx, artifactDir, workDir, name, doc, *story, contextFiles)
		if err != nil {
			_ = appendProgress(artifactDir, name, storyID, "error", "plan phase failed: "+err.Error())
			return fmt.Errorf("plan phase failed: %w", err)
		}
		// Re-read learnings after plan in case the agent added any.
		if m.compoundEnabled {
			contextFiles = m.buildContextFiles(artifactDir, workDir, name)
			contextFiles = m.injectLearnings(contextFiles, artifactDir, name)
		}
	}

	// Inject plan file into work context if it exists.
	if planPath != "" {
		contextFiles = append(contextFiles, planPath)
	}

	// ── PHASE 2: Work ─────────────────────────────────────────────────────────
	prompt := buildStoryPrompt(doc, *story)
	request := providers.IterationRequest{
		WorkDir:        workDir,
		Prompt:         prompt,
		ContextFiles:   contextFiles,
		ApprovalPolicy: m.iteration.ApprovalPolicy,
		SandboxPolicy:  m.iteration.SandboxPolicy,
		Model:          m.iteration.Model,
		Metadata: map[string]string{
			"storyID": storyID,
		},
	}

	result, iterationAttempt, err := m.runIterationWithRetry(ctx, artifactDir, name, request)
	if err != nil {
		_ = appendProgress(artifactDir, name, storyID, "error", result.Summary)
		_ = m.appendLearnings(artifactDir, name, storyID, "work", err.Error())
		return fmt.Errorf("iteration failed: %w", err)
	}

	// ── PHASE 3: Parallel Review (optional) ───────────────────────────────────
	if m.reviewer != nil && len(m.reviewPerspectives) > 0 {
		reviewReport, reviewErr := m.reviewer.RunReview(ctx, workDir, contextFiles, m.reviewPerspectives, request)
		if reviewErr != nil {
			_ = appendAgentLog(artifactDir, name, "[review] error: "+reviewErr.Error()+"\n")
		}
		summary := providers.SynthesizeReviewSummary(reviewReport.Reviews)
		if summary != "" {
			_ = appendAgentLog(artifactDir, name, "[review] summary:\n"+summary+"\n")
		}
		// If review found issues, treat as a quality failure.
		if !reviewReport.Passed {
			_ = appendProgress(artifactDir, name, storyID, "failed", "[review] "+summary)
			_ = m.appendLearnings(artifactDir, name, storyID, "review", summary)
			return fmt.Errorf("review found issues")
		}
	}

	// ── PHASE 4: Sequential Quality Checks ────────────────────────────────────
	if m.qualityChecker == nil {
		return fmt.Errorf("quality checker is not configured")
	}

	report, err := m.qualityChecker.Run(ctx, workDir, m.qualityCommands)
	if err != nil {
		_ = appendQualityRunnerError(artifactDir, name, storyID, iterationAttempt, err)
		_ = appendProgress(artifactDir, name, storyID, "error", err.Error())
		return fmt.Errorf("quality checks failed to run: %w", err)
	}
	if err := appendQualityReport(artifactDir, name, storyID, iterationAttempt, report); err != nil {
		return fmt.Errorf("failed to persist quality report: %w", err)
	}
	if !report.Passed {
		_ = appendProgress(artifactDir, name, storyID, "failed", formatQualitySummary(report))
		_ = m.appendLearnings(artifactDir, name, storyID, "quality", formatQualitySummary(report))
		return fmt.Errorf("quality checks failed")
	}

	// ── PHASE 5: Commit ────────────────────────────────────────────────────────
	if m.committer == nil {
		return fmt.Errorf("git committer is not configured")
	}

	commitResult, err := m.committer.CommitStory(ctx, workDir, storyID, storyTitle)
	if err != nil {
		_ = appendProgress(artifactDir, name, storyID, "error", err.Error())
		return fmt.Errorf("git commit failed: %w", err)
	}

	if err := markStoryPassed(&doc, storyID); err != nil {
		return err
	}
	if err := m.store.Save(name, doc); err != nil {
		return err
	}

	if m.completion.PushOnComplete && commitResult.Committed && m.completionExec != nil {
		if pushErr := m.completionExec.PushBranch(ctx, workDir); pushErr != nil {
			_ = appendAgentLog(artifactDir, name, "[completion] push failed: "+pushErr.Error()+"\n")
		} else if m.completion.AutoPROnComplete {
			if prErr := m.completionExec.CreatePR(ctx, workDir); prErr != nil {
				_ = appendAgentLog(artifactDir, name, "[completion] pr creation failed: "+prErr.Error()+"\n")
			}
		}
	}

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		summary = "story completed"
	}
	summary = summary + "\n\n" + formatQualitySummary(report)
	if !commitResult.Committed {
		summary = summary + " (no commit created)"
	}
	if err := appendProgress(artifactDir, name, storyID, "passed", summary); err != nil {
		return err
	}

	return nil
}

func (m Manager) runIterationWithRetry(ctx context.Context, artifactDir, name string, request providers.IterationRequest) (providers.IterationResult, int, error) {
	var lastErr error
	var lastResult providers.IterationResult
	lastAttempt := 0
	totalAttempts := m.retry.MaxRetries + 1
	if totalAttempts < 1 {
		totalAttempts = 1
	}

	for attempt := 0; attempt < totalAttempts; attempt++ {
		lastAttempt = attempt + 1
		events, result, err := m.provider.RunIteration(ctx, request)
		lastResult = result
		if err != nil {
			lastErr = err
			_ = appendProviderError(artifactDir, name, request.Metadata["storyID"], attempt+1, err)
			if !providers.IsRetryable(err) {
				return lastResult, lastAttempt, err
			}
		} else {
			summary, runtimeErr, consumeErr := consumeProviderEvents(artifactDir, name, request.Metadata["storyID"], attempt+1, events)
			if consumeErr != nil {
				return lastResult, lastAttempt, consumeErr
			}
			if summary != "" {
				lastResult.Summary = summary
			}
			if runtimeErr == nil {
				lastResult.Success = true
				return lastResult, lastAttempt, nil
			}
			lastErr = runtimeErr
			if !providers.IsRetryable(runtimeErr) {
				return lastResult, lastAttempt, runtimeErr
			}
		}

		if attempt == totalAttempts-1 {
			break
		}

		delay := m.retryDelay(attempt)
		if delay <= 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return lastResult, lastAttempt, ctx.Err()
		case <-time.After(delay):
		}
	}

	return lastResult, lastAttempt, lastErr
}

func (m Manager) retryDelay(attempt int) time.Duration {
	if len(m.retry.Delays) == 0 {
		return 0
	}
	if attempt < len(m.retry.Delays) {
		return m.retry.Delays[attempt]
	}
	return m.retry.Delays[len(m.retry.Delays)-1]
}

func setStoryInProgress(doc *prd.Document, storyID string) error {
	for i := range doc.UserStories {
		if doc.UserStories[i].ID == storyID {
			doc.UserStories[i].InProgress = true
			return nil
		}
	}
	return fmt.Errorf("story %q not found", storyID)
}

func markStoryPassed(doc *prd.Document, storyID string) error {
	for i := range doc.UserStories {
		if doc.UserStories[i].ID == storyID {
			doc.UserStories[i].Passes = true
			doc.UserStories[i].InProgress = false
			return nil
		}
	}
	return fmt.Errorf("story %q not found", storyID)
}

func consumeProviderEvents(workDir, name, storyID string, iteration int, events <-chan providers.Event) (summary string, runtimeErr error, err error) {
	if events == nil {
		return "", providers.NewConfigurationError("provider started without event stream", nil), nil
	}
	for event := range events {
		if err := appendEvent(workDir, name, storyID, iteration, event); err != nil {
			return "", nil, err
		}
		if err := appendAgentLog(workDir, name, fmt.Sprintf("[%s] %s\n", event.Type, event.Message)); err != nil {
			return "", nil, err
		}
		switch event.Type {
		case providers.EventAssistantText:
			summary = strings.TrimSpace(event.Message)
		case providers.EventError:
			runtimeErr = providers.DecodeEventError(event.Message)
		}
	}
	return summary, runtimeErr, nil
}

func appendProviderError(workDir, name, storyID string, iteration int, err error) error {
	event := providers.Event{Type: providers.EventError, Message: err.Error()}
	return appendEvent(workDir, name, storyID, iteration, event)
}

func appendQualityRunnerError(workDir, name, storyID string, iteration int, err error) error {
	payload := map[string]interface{}{
		"type":      string(providers.EventError),
		"message":   "quality checks failed to run: " + err.Error(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"iteration": iteration,
		"storyID":   storyID,
		"phase":     "quality",
	}
	if appendErr := appendEventPayload(workDir, name, payload); appendErr != nil {
		return appendErr
	}
	return appendAgentLog(workDir, name, "[quality] runner error: "+err.Error()+"\n")
}

func appendEvent(workDir, name, storyID string, iteration int, event providers.Event) (err error) {
	payload := map[string]interface{}{
		"type":      string(event.Type),
		"message":   event.Message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"iteration": iteration,
		"storyID":   storyID,
	}
	return appendEventPayload(workDir, name, payload)
}

func appendQualityReport(workDir, name, storyID string, iteration int, report quality.Report) error {
	for _, result := range report.Results {
		payload := map[string]interface{}{
			"type":      string(providers.EventCommandOutput),
			"message":   fmt.Sprintf("quality command %q completed with exit code %d", result.Command, result.ExitCode),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"iteration": iteration,
			"storyID":   storyID,
			"phase":     "quality",
			"command":   result.Command,
			"exitCode":  result.ExitCode,
			"duration":  result.Duration.String(),
			"stdout":    result.Stdout,
			"stderr":    result.Stderr,
			"passed":    result.ExitCode == 0,
		}
		if err := appendEventPayload(workDir, name, payload); err != nil {
			return err
		}

		status := "passed"
		if result.ExitCode != 0 {
			status = "failed"
		}
		line := fmt.Sprintf("[quality] %s (%s) -> exit=%d duration=%s\n", result.Command, status, result.ExitCode, result.Duration)
		if err := appendAgentLog(workDir, name, line); err != nil {
			return err
		}
	}
	return nil
}

func appendEventPayload(workDir, name string, payload map[string]interface{}) (err error) {
	path := project.PRDEventsPath(workDir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = file.Write(append(data, '\n'))
	return err
}

func appendAgentLog(workDir, name, line string) (err error) {
	path := project.PRDAgentLogPath(workDir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	_, err = file.WriteString(line)
	return err
}

func appendProgress(workDir, name, storyID, status, summary string) (err error) {
	path := project.PRDProgressPath(workDir, name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}()

	block := fmt.Sprintf(
		"\n## Iteration - %s - %s\nDate: %s\n\nSummary:\n%s\n",
		storyID,
		status,
		time.Now().UTC().Format(time.RFC3339),
		summary,
	)
	_, err = file.WriteString(block)
	return err
}

func formatQualitySummary(report quality.Report) string {
	builder := strings.Builder{}
	builder.WriteString("Quality checks:\n")
	for _, result := range report.Results {
		builder.WriteString("- Command: ")
		builder.WriteString(result.Command)
		builder.WriteString("\n")
		builder.WriteString("  Exit code: ")
		builder.WriteString(strconv.Itoa(result.ExitCode))
		builder.WriteString("\n")
		builder.WriteString("  Duration: ")
		builder.WriteString(result.Duration.String())
		builder.WriteString("\n")
		builder.WriteString("  Stdout:\n")
		builder.WriteString(indentedBlock(strings.TrimSpace(result.Stdout)))
		builder.WriteString("\n")
		builder.WriteString("  Stderr:\n")
		builder.WriteString(indentedBlock(strings.TrimSpace(result.Stderr)))
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func indentedBlock(text string) string {
	if text == "" {
		return "    (empty)"
	}
	lines := strings.Split(text, "\n")
	for i := range lines {
		lines[i] = "    " + lines[i]
	}
	return strings.Join(lines, "\n")
}

// buildContextFiles assembles the ordered list of context files for an iteration.
// This is a method on Manager so it can be reused in the plan phase.
func (m Manager) buildContextFiles(artifactDir, workDir, prdName string) []string {
	candidates := []string{
		project.PRDMarkdownPath(artifactDir, prdName),
		project.PRDJSONPath(artifactDir, prdName),
		project.PRDProgressPath(artifactDir, prdName),
	}

	for _, optionalArtifact := range []string{
		project.PRDProjectSummaryPath(artifactDir, prdName),
		project.PRDJTBDPath(artifactDir, prdName),
		project.PRDArchitecturePath(artifactDir, prdName),
	} {
		if _, err := os.Stat(optionalArtifact); err == nil {
			candidates = append(candidates, optionalArtifact)
		}
	}

	for _, optional := range []string{"AGENTS.md", "README.md"} {
		optionalPath := filepath.Join(workDir, optional)
		if _, err := os.Stat(optionalPath); err == nil {
			candidates = append(candidates, optionalPath)
		}
	}

	seen := map[string]struct{}{}
	contextFiles := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		normalized, ok := normalizeContextPath(workDir, candidate)
		if !ok {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		contextFiles = append(contextFiles, normalized)
	}

	return contextFiles
}

// injectLearnings appends the learnings file to the context files if it exists.
func (m Manager) injectLearnings(contextFiles []string, artifactDir, prdName string) []string {
	path := project.PRDLearningsPath(artifactDir, prdName)
	if _, err := os.Stat(path); err != nil {
		return contextFiles
	}
	result := make([]string, 0, len(contextFiles)+1)
	result = append(result, contextFiles...)
	result = append(result, path)
	return result
}

// runPlanPhase generates an implementation plan for the given story and writes it
// to .daedalus/prds/<name>/plans/<storyID>.md. Returns the plan path on success.
func (m Manager) runPlanPhase(
	ctx context.Context,
	artifactDir, workDir, prdName string,
	doc prd.Document,
	story prd.UserStory,
	contextFiles []string,
) (string, error) {
	// Ensure plans directory exists.
	plansDir := project.PRDPlansDir(artifactDir, prdName)
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create plans directory: %w", err)
	}

	planPath := project.PRDPlanPath(artifactDir, prdName, story.ID)
	prompt := buildPlanPrompt(doc, story)

	request := providers.IterationRequest{
		WorkDir:        workDir,
		Prompt:         prompt,
		ContextFiles:   contextFiles,
		ApprovalPolicy: m.iteration.ApprovalPolicy,
		SandboxPolicy:  m.iteration.SandboxPolicy,
		Model:          m.iteration.Model,
		Metadata: map[string]string{
			"storyID": story.ID,
			"phase":   "plan",
		},
	}

	events, result, err := m.provider.RunIteration(ctx, request)
	if err != nil {
		return "", fmt.Errorf("plan phase provider error: %w", err)
	}

	var planText strings.Builder
	for event := range events {
		if event.Type == providers.EventAssistantText {
			planText.WriteString(event.Message)
		}
	}

	planContent := strings.TrimSpace(planText.String())
	if planContent == "" {
		planContent = strings.TrimSpace(result.Summary)
	}

	if err := os.WriteFile(planPath, []byte(planContent), 0o644); err != nil {
		return "", fmt.Errorf("failed to write plan file: %w", err)
	}

	_ = appendAgentLog(artifactDir, prdName, "[plan] wrote "+planPath+"\n")
	return planPath, nil
}

// buildPlanPrompt generates the prompt for the plan phase.
func buildPlanPrompt(doc prd.Document, story prd.UserStory) string {
	builder := strings.Builder{}
	builder.WriteString("You are a senior engineer creating an implementation plan.\n\n")
	builder.WriteString("Project: ")
	builder.WriteString(strings.TrimSpace(doc.Project))
	builder.WriteString("\n\nProject Description: ")
	builder.WriteString(strings.TrimSpace(doc.Description))
	builder.WriteString("\n\nActive Story\n")
	builder.WriteString("ID: ")
	builder.WriteString(story.ID)
	builder.WriteString("\nTitle: ")
	builder.WriteString(story.Title)
	builder.WriteString("\nDescription: ")
	builder.WriteString(story.Description)
	builder.WriteString("\nPriority: ")
	builder.WriteString(strconv.Itoa(story.Priority))
	builder.WriteString("\nAcceptance Criteria:\n")
	for _, criterion := range story.AcceptanceCriteria {
		builder.WriteString("- ")
		builder.WriteString(criterion)
		builder.WriteString("\n")
	}
	builder.WriteString("\nYour task: Write a detailed implementation plan for this story.\n")
	builder.WriteString("Cover: objective, proposed architecture, implementation approach, key files to modify, potential pitfalls, and success criteria.\n")
	builder.WriteString("Output a clean markdown plan. Do not implement the code.\n")
	return builder.String()
}

// appendLearnings records a learning entry in the learnings file after a failure.
// The learnings file accumulates across the project's lifetime.
func (m Manager) appendLearnings(artifactDir, prdName, storyID, phase, summary string) error {
	if !m.compoundEnabled {
		return nil
	}
	path := project.PRDLearningsPath(artifactDir, prdName)
	entry := fmt.Sprintf(
		"\n## %s — %s [%s]\n%s\n",
		storyID,
		phase,
		time.Now().UTC().Format(time.RFC3339),
		summary,
	)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(entry)
	return err
}

func buildStoryPrompt(doc prd.Document, story prd.UserStory) string {
	builder := strings.Builder{}
	builder.WriteString("Project: ")
	builder.WriteString(strings.TrimSpace(doc.Project))
	builder.WriteString("\n")
	builder.WriteString("Project Description: ")
	builder.WriteString(strings.TrimSpace(doc.Description))
	builder.WriteString("\n\n")
	builder.WriteString("Active Story\n")
	builder.WriteString("ID: ")
	builder.WriteString(story.ID)
	builder.WriteString("\n")
	builder.WriteString("Title: ")
	builder.WriteString(story.Title)
	builder.WriteString("\n")
	builder.WriteString("Description: ")
	builder.WriteString(story.Description)
	builder.WriteString("\n")
	builder.WriteString("Priority: ")
	builder.WriteString(strconv.Itoa(story.Priority))
	builder.WriteString("\n")
	builder.WriteString("Acceptance Criteria:\n")
	for _, criterion := range story.AcceptanceCriteria {
		builder.WriteString("- ")
		builder.WriteString(criterion)
		builder.WriteString("\n")
	}
	builder.WriteString("\nRules:\n")
	builder.WriteString("- Implement only this active story.\n")
	builder.WriteString("- Satisfy all acceptance criteria.\n")
	builder.WriteString("- Do not execute destructive git operations.\n")
	builder.WriteString("- Summarize code changes and check results.\n")

	return strings.TrimSpace(builder.String())
}

func normalizeContextPath(baseDir, candidate string) (string, bool) {
	if strings.TrimSpace(candidate) == "" {
		return "", false
	}
	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return filepath.Clean(candidate), true
	}

	baseAbs, baseErr := filepath.Abs(baseDir)
	if baseErr != nil {
		return absPath, true
	}
	relative, relErr := filepath.Rel(baseAbs, absPath)
	if relErr == nil && relative != "." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != ".." {
		return filepath.ToSlash(relative), true
	}
	return absPath, true
}
