package providers

import (
	"context"
	"strings"
)

// RunIterationSimple is a convenience wrapper that runs a single iteration
// using an already-resolved Provider. It is intended for short-lived sub-tasks
// such as parallel review perspectives.
func RunIterationSimple(ctx context.Context, provider Provider, request IterationRequest) (<-chan Event, IterationResult, error) {
	if provider == nil {
		return nil, IterationResult{}, nil
	}
	return provider.RunIteration(ctx, request)
}

// SynthesizeReviewSummary concatenates all findings from a set of perspective reviews
// into a human-readable string.
func SynthesizeReviewSummary(reviews []PerspectiveReview) string {
	if len(reviews) == 0 {
		return "No review perspectives ran."
	}
	var b strings.Builder
	for _, r := range reviews {
		if r.Error != "" {
			b.WriteString("[")
			b.WriteString(r.Perspective)
			b.WriteString("] error: ")
			b.WriteString(r.Error)
			b.WriteString("\n")
		}
		if len(r.Findings) > 0 {
			b.WriteString("[")
			b.WriteString(r.Perspective)
			b.WriteString("] (")
			b.WriteString(r.Duration)
			b.WriteString("):\n")
			for _, f := range r.Findings {
				b.WriteString("  - ")
				b.WriteString(f)
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}
