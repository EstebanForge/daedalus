package onboarding

import "time"

// ProjectMode describes the state of the working directory at onboarding time.
type ProjectMode string

const (
	ProjectModeEmpty    ProjectMode = "empty_folder"
	ProjectModeExisting ProjectMode = "existing_project"
)

// Steps tracks which onboarding steps have been completed.
type Steps struct {
	GitIgnore        bool `json:"git_ignore"`
	ProjectDiscovery bool `json:"project_discovery"`
	JTBD             bool `json:"jtbd"`
	CreatePRD        bool `json:"create_prd"`
}

// State is the persisted onboarding state stored in .daedalus/onboarding/state.json.
type State struct {
	Completed   bool        `json:"completed"`
	ProjectMode ProjectMode `json:"project_mode"`
	Steps       Steps       `json:"steps"`
	UpdatedAt   time.Time   `json:"updated_at"`
}
