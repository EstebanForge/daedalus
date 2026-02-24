package onboarding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EstebanForge/daedalus/internal/project"
)

func TestIsRequired_MissingFile(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	required, err := mgr.IsRequired()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !required {
		t.Error("expected required=true when state file is absent")
	}
}

func TestIsRequired_CompletedFalse(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	state := State{Completed: false}
	if err := mgr.SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	required, err := mgr.IsRequired()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !required {
		t.Error("expected required=true when completed=false")
	}
}

func TestIsRequired_Complete(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	state := State{Completed: true}
	if err := mgr.SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	required, err := mgr.IsRequired()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if required {
		t.Error("expected required=false when completed=true")
	}
}

func TestDetectProjectMode_Empty(t *testing.T) {
	dir := t.TempDir()

	// Create only the .daedalus directory — everything else absent.
	if err := os.MkdirAll(filepath.Join(dir, project.DirectoryName), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	mgr := NewManager(dir)
	mode, err := mgr.DetectProjectMode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != ProjectModeEmpty {
		t.Errorf("expected %q, got %q", ProjectModeEmpty, mode)
	}
}

func TestDetectProjectMode_Existing(t *testing.T) {
	dir := t.TempDir()

	// Write a file other than .daedalus.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	mgr := NewManager(dir)
	mode, err := mgr.DetectProjectMode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != ProjectModeExisting {
		t.Errorf("expected %q, got %q", ProjectModeExisting, mode)
	}
}

func TestFirstIncompleteStep_ExistingProject(t *testing.T) {
	mgr := NewManager(t.TempDir())

	state := State{
		ProjectMode: ProjectModeExisting,
		Steps:       Steps{},
	}

	step := mgr.FirstIncompleteStep(state)
	if step != "git_ignore" {
		t.Errorf("expected %q, got %q", "git_ignore", step)
	}
}

func TestFirstIncompleteStep_EmptyFolder_SkipsDiscovery(t *testing.T) {
	mgr := NewManager(t.TempDir())

	state := State{
		ProjectMode: ProjectModeEmpty,
		Steps: Steps{
			GitIgnore: true,
			// ProjectDiscovery intentionally not done
		},
	}

	step := mgr.FirstIncompleteStep(state)
	if step != "jtbd" {
		t.Errorf("expected %q (discovery skipped in empty mode), got %q", "jtbd", step)
	}
}

func TestFirstIncompleteStep_AllDone(t *testing.T) {
	mgr := NewManager(t.TempDir())

	state := State{
		ProjectMode: ProjectModeExisting,
		Steps: Steps{
			GitIgnore:        true,
			ProjectDiscovery: true,
			JTBD:             true,
			CreatePRD:        true,
		},
	}

	step := mgr.FirstIncompleteStep(state)
	if step != "" {
		t.Errorf("expected empty string when all done, got %q", step)
	}
}

func TestSaveLoadState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	original := State{
		Completed:   false,
		ProjectMode: ProjectModeExisting,
		Steps: Steps{
			GitIgnore:        true,
			ProjectDiscovery: false,
			JTBD:             false,
			CreatePRD:        false,
		},
		UpdatedAt: time.Time{}, // will be overwritten by SaveState
	}

	if err := mgr.SaveState(original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := mgr.LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	if loaded.Completed != original.Completed {
		t.Errorf("Completed: got %v, want %v", loaded.Completed, original.Completed)
	}
	if loaded.ProjectMode != original.ProjectMode {
		t.Errorf("ProjectMode: got %q, want %q", loaded.ProjectMode, original.ProjectMode)
	}
	if loaded.Steps != original.Steps {
		t.Errorf("Steps: got %+v, want %+v", loaded.Steps, original.Steps)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set by SaveState")
	}
}

func TestSaveState_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	state := State{Completed: true}
	if err := mgr.SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// temp file should not remain
	tmp := project.OnboardingStatePath(dir) + ".tmp"
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("temp file should not exist after SaveState")
	}

	// state file should exist and be valid JSON
	data, err := os.ReadFile(project.OnboardingStatePath(dir))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var check State
	if err := json.Unmarshal(data, &check); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestLoadState_DefaultWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)

	state, err := mgr.LoadState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Completed {
		t.Error("default state should have Completed=false")
	}
	if state.ProjectMode != "" {
		t.Errorf("default state should have empty ProjectMode, got %q", state.ProjectMode)
	}
}
