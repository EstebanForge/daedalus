package prd

import (
	"fmt"
	"strings"
)

type ValidationResult struct {
	Errors []string
}

func (v ValidationResult) Valid() bool {
	return len(v.Errors) == 0
}

func Validate(doc Document) ValidationResult {
	result := ValidationResult{}
	if strings.TrimSpace(doc.Project) == "" {
		result.Errors = append(result.Errors, "project is required")
	}
	if len(doc.UserStories) == 0 {
		result.Errors = append(result.Errors, "at least one user story is required")
		return result
	}

	ids := make(map[string]struct{}, len(doc.UserStories))
	priorities := make(map[int]struct{}, len(doc.UserStories))

	for i, story := range doc.UserStories {
		prefix := fmt.Sprintf("userStories[%d]", i)
		if strings.TrimSpace(story.ID) == "" {
			result.Errors = append(result.Errors, prefix+": id is required")
		}
		if strings.TrimSpace(story.Title) == "" {
			result.Errors = append(result.Errors, prefix+": title is required")
		}
		if strings.TrimSpace(story.Description) == "" {
			result.Errors = append(result.Errors, prefix+": description is required")
		}
		if story.Priority < 1 {
			result.Errors = append(result.Errors, prefix+": priority must be >= 1")
		}
		if len(story.AcceptanceCriteria) == 0 {
			result.Errors = append(result.Errors, prefix+": acceptanceCriteria must not be empty")
		}

		if _, exists := ids[story.ID]; exists {
			result.Errors = append(result.Errors, prefix+": duplicate id "+story.ID)
		}
		ids[story.ID] = struct{}{}

		if _, exists := priorities[story.Priority]; exists {
			result.Errors = append(result.Errors, prefix+": duplicate priority")
		}
		priorities[story.Priority] = struct{}{}
	}

	return result
}
