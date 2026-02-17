package project

import "path/filepath"

const (
	DirectoryName = ".daedalus"
	PRDsDirectory = "prds"
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
