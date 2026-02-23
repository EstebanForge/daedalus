package providers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

func runCLIStreaming(ctx context.Context, command string, args []string, workDir string, events chan Event) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if strings.TrimSpace(workDir) != "" {
		cmd.Dir = workDir
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var mu sync.Mutex
	var output strings.Builder
	appendOutput := func(text string) {
		mu.Lock()
		defer mu.Unlock()
		if text == "" {
			return
		}
		if output.Len() > 0 {
			output.WriteByte('\n')
		}
		output.WriteString(text)
	}

	var wg sync.WaitGroup
	stream := func(reader io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		// Increase max token size for large CLI output lines.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			appendOutput(line)
			pushProviderEvent(events, EventCommandOutput, line)
		}
	}

	wg.Add(2)
	go stream(stdoutPipe)
	go stream(stderrPipe)

	waitErr := cmd.Wait()
	wg.Wait()

	mu.Lock()
	text := strings.TrimSpace(output.String())
	mu.Unlock()
	if waitErr == nil {
		return text, nil
	}

	if text == "" {
		return "", waitErr
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return "", fmt.Errorf("%w: %s", waitErr, text)
	}
	return "", fmt.Errorf("%w: %s", waitErr, text)
}
