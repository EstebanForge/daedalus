package quality

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/EstebanForge/daedalus/internal/providers"
)

// Reviewer runs agent-powered code reviews across multiple perspectives.
type Reviewer interface {
	RunReview(ctx context.Context, workDir string, contextFiles []string, perspectives []string, baseOpts providers.IterationRequest) (ReviewReport, error)
}

// ReviewReport aggregates results from all review perspectives.
type ReviewReport struct {
	Passed  bool
	Reviews []providers.PerspectiveReview
}

// parallelReviewer runs multiple review perspectives in parallel via goroutines.
type parallelReviewer struct {
	provider providers.Provider
}

// NewParallelReviewer creates a Reviewer that runs perspectives concurrently.
// Pass the ACP provider that will be used to run review iterations.
func NewParallelReviewer(provider providers.Provider) Reviewer {
	return &parallelReviewer{provider: provider}
}

// RunReview executes each perspective as a separate agent iteration in parallel,
// then aggregates findings into a ReviewReport.
func (r *parallelReviewer) RunReview(
	ctx context.Context,
	workDir string,
	contextFiles []string,
	perspectives []string,
	baseOpts providers.IterationRequest,
) (ReviewReport, error) {
	if len(perspectives) == 0 {
		return ReviewReport{Passed: true}, nil
	}

	type result struct {
		review providers.PerspectiveReview
		err    error
	}
	ch := make(chan result, len(perspectives))
	var wg sync.WaitGroup

	for _, perspective := range perspectives {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			review := r.runPerspective(ctx, workDir, contextFiles, p, baseOpts)
			ch <- result{review: review}
		}(perspective)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var reviews []providers.PerspectiveReview
	var firstErr error
	for res := range ch {
		if res.err != nil && firstErr == nil {
			firstErr = res.err
		}
		reviews = append(reviews, res.review)
	}

	allPassed := true
	for _, rev := range reviews {
		if len(rev.Findings) > 0 || rev.Error != "" {
			allPassed = false
			break
		}
	}

	return ReviewReport{
		Passed:  allPassed,
		Reviews: reviews,
	}, firstErr
}

func (r *parallelReviewer) runPerspective(
	ctx context.Context,
	workDir string,
	contextFiles []string,
	perspective string,
	baseOpts providers.IterationRequest,
) providers.PerspectiveReview {
	startedAt := time.Now()
	prompt := buildPerspectivePrompt(workDir, perspective)

	request := providers.IterationRequest{
		WorkDir:        workDir,
		Prompt:         prompt,
		ContextFiles:   contextFiles,
		ApprovalPolicy: baseOpts.ApprovalPolicy,
		SandboxPolicy:  baseOpts.SandboxPolicy,
		Model:          baseOpts.Model,
		Metadata: map[string]string{
			"storyID":     baseOpts.Metadata["storyID"],
			"phase":       "review",
			"perspective": perspective,
		},
	}

	events, result, err := providers.RunIterationSimple(ctx, r.provider, request)
	review := providers.PerspectiveReview{
		Perspective: perspective,
		Duration:    time.Since(startedAt).String(),
	}

	if err != nil {
		review.Error = err.Error()
		return review
	}

	var summary strings.Builder
	for event := range events {
		if event.Type == providers.EventAssistantText {
			summary.WriteString(event.Message)
		}
	}

	output := strings.TrimSpace(summary.String())
	if output == "" {
		return review
	}

	// Parse findings. Lines starting with "-" or "*" are treated as findings.
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			review.Findings = append(review.Findings, strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* "))
		} else {
			review.Findings = append(review.Findings, line)
		}
	}

	_ = result // available for future fine-grained reporting
	return review
}

// buildPerspectivePrompt returns a review prompt for the given perspective.
func buildPerspectivePrompt(workDir string, perspective string) string {
	var focus string
	switch perspective {
	case "security":
		focus = "common security issues: SQL injection, hardcoded secrets, unsafe shell commands, missing input validation, exposed credentials in logs"
	case "performance":
		focus = "common performance issues: N+1 queries, missing database indexes, unbounded loops over large datasets, unnecessary allocations, synchronous blocking I/O"
	case "complexity":
		focus = "over-engineering and unnecessary complexity: redundant abstractions, premature optimization, over-engineered patterns, bloated boilerplate, code that is harder to maintain than it needs to be"
	default:
		focus = "code quality issues"
	}

	return fmt.Sprintf(`You are reviewing code changes in %s for the "%s" perspective.

Review the modified files and identify any issues related to: %s.

Output format: Write each finding on its own line starting with "- ". If no issues are found, output nothing. Be specific and concise. Do not include header text, explanations, or wrapping text outside the bullet list.`, workDir, perspective, focus)
}
