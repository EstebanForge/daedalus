package providers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
)

const acpDoctorTimeout = 15 * time.Second

type ACPDoctorCheck struct {
	ProviderKey  string
	Enabled      bool
	Command      string
	BinaryPath   string
	Healthy      bool
	Capabilities Capabilities
	Message      string
}

type ACPDoctorReport struct {
	Checks []ACPDoctorCheck
}

func (r ACPDoctorReport) Healthy() bool {
	for _, check := range r.Checks {
		if check.Enabled && !check.Healthy {
			return false
		}
	}
	return true
}

// RunACPDoctor runs lightweight ACP health probes for target providers.
func RunACPDoctor(ctx context.Context, cfg config.Config, targets []string) (ACPDoctorReport, error) {
	keys, err := resolveDoctorTargets(targets)
	if err != nil {
		return ACPDoctorReport{}, err
	}

	report := ACPDoctorReport{
		Checks: make([]ACPDoctorCheck, 0, len(keys)),
	}

	workingDir, wdErr := os.Getwd()
	if wdErr != nil {
		workingDir = "."
	}

	for _, key := range keys {
		providerCfg := getProviderConfig(cfg, key)
		enabled := providerEnabled(cfg, key)
		command := resolveACPCommand(key, providerCfg.ACPCommand)
		commandText := strings.TrimSpace(strings.Join(append([]string{command.Binary}, command.Args...), " "))

		check := ACPDoctorCheck{
			ProviderKey: key,
			Enabled:     enabled,
			Command:     commandText,
			Healthy:     false,
		}
		if !enabled {
			check.Message = "provider disabled"
			report.Checks = append(report.Checks, check)
			continue
		}

		binaryPath, lookupErr := exec.LookPath(command.Binary)
		if lookupErr != nil {
			check.Message = fmt.Sprintf("acp binary %q not found in PATH", command.Binary)
			report.Checks = append(report.Checks, check)
			continue
		}
		check.BinaryPath = binaryPath

		provider, ok := newACPProvider(cfg, key).(acpProvider)
		if !ok {
			check.Message = "internal error: expected acp provider"
			report.Checks = append(report.Checks, check)
			continue
		}

		session, startErr := provider.startSession(workingDir)
		if startErr != nil {
			check.Message = startErr.Error()
			report.Checks = append(report.Checks, check)
			continue
		}

		probeCtx, cancel := context.WithTimeout(ctx, acpDoctorTimeout)
		initErr := provider.initializeTransport(probeCtx, session)
		if initErr == nil {
			initErr = provider.createSession(probeCtx, session, workingDir)
		}
		cancel()
		provider.closeSession(session)

		if initErr != nil {
			check.Message = initErr.Error()
			report.Checks = append(report.Checks, check)
			continue
		}

		check.Healthy = true
		check.Capabilities = session.getCapabilities()
		check.Message = "initialize and session/new succeeded"
		report.Checks = append(report.Checks, check)
	}

	return report, nil
}

func resolveDoctorTargets(targets []string) ([]string, error) {
	if len(targets) == 0 {
		return KnownProviderKeys(), nil
	}

	known := make(map[string]struct{}, len(knownProviderKeys))
	for _, key := range knownProviderKeys {
		known[key] = struct{}{}
	}

	resolved := make([]string, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, raw := range targets {
		key := strings.ToLower(strings.TrimSpace(raw))
		if key == "" {
			continue
		}
		if _, ok := known[key]; !ok {
			return nil, NewUnknownProviderError(key)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		resolved = append(resolved, key)
	}
	if len(resolved) == 0 {
		return nil, NewConfigurationError("doctor requires at least one provider key", nil)
	}
	return resolved, nil
}
