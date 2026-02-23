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

type qualityChecker interface {
	Run(ctx context.Context, workDir string, commands []string) (quality.Report, error)
}

type committer interface {
	CommitStory(ctx context.Context, workDir, storyID, storyTitle string) (daedalusgit.CommitResult, error)
}

type Manager struct {
	store           prd.Store
	provider        providers.Provider
	retry           RetryPolicy
	qualityChecker  qualityChecker
	qualityCommands []string
	committer       committer
}

func NewManager(
	store prd.Store,
	provider providers.Provider,
	retry RetryPolicy,
	checker qualityChecker,
	qualityCommands []string,
	commitService committer,
) Manager {
	return Manager{
		store:           store,
		provider:        provider,
		retry:           retry,
		qualityChecker:  checker,
		qualityCommands: qualityCommands,
		committer:       commitService,
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

	prompt := buildStoryPrompt(doc, *story)
	request := providers.IterationRequest{
		WorkDir:        workDir,
		Prompt:         prompt,
		ContextFiles:   buildContextFiles(artifactDir, workDir, name),
		ApprovalPolicy: "on-failure",
		SandboxPolicy:  "workspace-write",
		Model:          "default",
		Metadata: map[string]string{
			"storyID": storyID,
		},
	}

	result, iterationAttempt, err := m.runIterationWithRetry(ctx, artifactDir, name, request)
	if err != nil {
		_ = appendProgress(artifactDir, name, storyID, "error", result.Summary)
		return fmt.Errorf("iteration failed: %w", err)
	}

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
		return fmt.Errorf("quality checks failed")
	}

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

func buildContextFiles(artifactDir, workDir, prdName string) []string {
	candidates := []string{
		project.PRDMarkdownPath(artifactDir, prdName),
		project.PRDJSONPath(artifactDir, prdName),
		project.PRDProgressPath(artifactDir, prdName),
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
