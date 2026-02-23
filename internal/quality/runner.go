package quality

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Result struct {
	Command  string
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

type Report struct {
	Passed  bool
	Results []Result
}

type Runner struct{}

func NewRunner() Runner {
	return Runner{}
}

func (Runner) Run(ctx context.Context, workDir string, commands []string) (Report, error) {
	if len(commands) == 0 {
		return Report{}, fmt.Errorf("no quality commands configured")
	}

	report := Report{
		Passed:  true,
		Results: make([]Result, 0, len(commands)),
	}

	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			return Report{}, fmt.Errorf("quality command must not be empty")
		}

		result, err := runCommand(ctx, workDir, command)
		if err != nil {
			return Report{}, fmt.Errorf("failed to run quality command %q: %w", command, err)
		}

		report.Results = append(report.Results, result)
		if result.ExitCode != 0 {
			report.Passed = false
		}
	}

	return report, nil
}

func runCommand(ctx context.Context, workDir string, command string) (Result, error) {
	startedAt := time.Now()
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := Result{
		Command:  command,
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(startedAt),
	}

	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	return result, err
}
