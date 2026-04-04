package project

import "path/filepath"

const (
	DirectoryName = ".daedalus"
	PRDsDirectory = "prds"
	WorktreesDir  = "worktrees"
)

func PRDsPath(baseDir string) string {
	return filepath.Join(baseDir, DirectoryName, PRDsDirectory)
}

func PRDPath(baseDir, name string) string {
	return filepath.Join(PRDsPath(baseDir), name)
}

func PRDMarkdownPath(baseDir, name string) string {
	return filepath.Join(PRDPath(baseDir, name), "prd.md")
}

func PRDJSONPath(baseDir, name string) string {
	return filepath.Join(PRDPath(baseDir, name), "prd.json")
}

func PRDProgressPath(baseDir, name string) string {
	return filepath.Join(PRDPath(baseDir, name), "progress.md")
}

func PRDAgentLogPath(baseDir, name string) string {
	return filepath.Join(PRDPath(baseDir, name), "agent.log")
}

func PRDEventsPath(baseDir, name string) string {
	return filepath.Join(PRDPath(baseDir, name), "events.jsonl")
}

func WorktreesPath(baseDir string) string {
	return filepath.Join(baseDir, DirectoryName, WorktreesDir)
}

func WorktreePath(baseDir, name string) string {
	return filepath.Join(WorktreesPath(baseDir), name)
}

const OnboardingDirectory = "onboarding"
const ACPSessionsFile = "acp-sessions.json"

func OnboardingPath(workDir string) string {
	return filepath.Join(workDir, DirectoryName, OnboardingDirectory)
}

func OnboardingStatePath(workDir string) string {
	return filepath.Join(OnboardingPath(workDir), "state.json")
}

func ACPSessionsPath(workDir string) string {
	return filepath.Join(workDir, DirectoryName, ACPSessionsFile)
}

func PRDProjectSummaryPath(workDir, name string) string {
	return filepath.Join(PRDPath(workDir, name), "project-summary.md")
}

func PRDJTBDPath(workDir, name string) string {
	return filepath.Join(PRDPath(workDir, name), "jtbd.md")
}

func PRDArchitecturePath(workDir, name string) string {
	return filepath.Join(PRDPath(workDir, name), "architecture-design.md")
}

func PRDPlansDir(workDir, name string) string {
	return filepath.Join(PRDPath(workDir, name), "plans")
}

func PRDPlanPath(workDir, name, storyID string) string {
	return filepath.Join(PRDPlansDir(workDir, name), storyID+".md")
}

func PRDLearningsPath(workDir, name string) string {
	return filepath.Join(PRDPath(workDir, name), "learnings.md")
}
