package prd

type UserStory struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
	Priority           int      `json:"priority"`
	Passes             bool     `json:"passes"`
	InProgress         bool     `json:"inProgress,omitempty"`
}

type Document struct {
	Project     string      `json:"project"`
	Description string      `json:"description"`
	UserStories []UserStory `json:"userStories"`
}

func (d Document) CountComplete() int {
	count := 0
	for _, story := range d.UserStories {
		if story.Passes {
			count++
		}
	}
	return count
}

func (d Document) CountInProgress() int {
	count := 0
	for _, story := range d.UserStories {
		if story.InProgress {
			count++
		}
	}
	return count
}

func (d Document) NextStory() *UserStory {
	for i := range d.UserStories {
		if d.UserStories[i].InProgress {
			return &d.UserStories[i]
		}
	}

	var next *UserStory
	for i := range d.UserStories {
		story := &d.UserStories[i]
		if story.Passes {
			continue
		}
		if next == nil || story.Priority < next.Priority {
			next = story
		}
	}
	return next
}
