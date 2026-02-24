package onboarding

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/EstebanForge/daedalus/internal/project"
)

// Manager handles onboarding state for a working directory.
type Manager struct {
	workDir string
}

// NewManager returns a Manager for the given working directory.
func NewManager(workDir string) *Manager {
	return &Manager{workDir: workDir}
}

// IsRequired returns true when onboarding has not been completed:
// the state file is absent, or completed=false.
func (m *Manager) IsRequired() (bool, error) {
	path := project.OnboardingStatePath(m.workDir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return true, nil
	}
	state, err := m.LoadState()
	if err != nil {
		return false, err
	}
	return !state.Completed, nil
}

// LoadState reads the onboarding state file. If the file is absent it returns
// a zero-value State (not an error).
func (m *Manager) LoadState() (State, error) {
	path := project.OnboardingStatePath(m.workDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, fmt.Errorf("reading onboarding state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("parsing onboarding state: %w", err)
	}
	return state, nil
}

// SaveState persists the onboarding state atomically via a temp-file rename.
func (m *Manager) SaveState(s State) error {
	dir := project.OnboardingPath(m.workDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating onboarding directory: %w", err)
	}

	s.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling onboarding state: %w", err)
	}
	data = append(data, '\n')

	path := project.OnboardingStatePath(m.workDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp onboarding state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renaming onboarding state: %w", err)
	}
	return nil
}

// DetectProjectMode inspects the working directory to determine whether it is
// effectively empty (only .daedalus/ present) or an existing project.
func (m *Manager) DetectProjectMode() (ProjectMode, error) {
	entries, err := os.ReadDir(m.workDir)
	if err != nil {
		return "", fmt.Errorf("reading working directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == project.DirectoryName {
			continue
		}
		return ProjectModeExisting, nil
	}
	return ProjectModeEmpty, nil
}

// FirstIncompleteStep returns the name of the first onboarding step that has
// not yet been completed, or "" when all steps are done.
// "project_discovery" is skipped when the project mode is empty_folder.
func (m *Manager) FirstIncompleteStep(s State) string {
	if !s.Steps.GitIgnore {
		return "git_ignore"
	}
	if s.ProjectMode != ProjectModeEmpty && !s.Steps.ProjectDiscovery {
		return "project_discovery"
	}
	if !s.Steps.JTBD {
		return "jtbd"
	}
	if !s.Steps.CreatePRD {
		return "create_prd"
	}
	return ""
}
