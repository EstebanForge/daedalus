package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Provider  ProviderConfig  `toml:"provider"`
	Retry     RetryConfig     `toml:"retry"`
	Providers ProvidersConfig `toml:"providers"`
}

type ProviderConfig struct {
	Default string `toml:"default"`
}

type RetryConfig struct {
	MaxRetries int      `toml:"max_retries"`
	Delays     []string `toml:"delays"`
}

type ProvidersConfig struct {
	Codex  GenericProviderConfig `toml:"codex"`
	Claude GenericProviderConfig `toml:"claude"`
	Gemini GenericProviderConfig `toml:"gemini"`
}

type GenericProviderConfig struct {
	Enabled        bool   `toml:"enabled"`
	Model          string `toml:"model"`
	ApprovalPolicy string `toml:"approval_policy"`
	SandboxPolicy  string `toml:"sandbox_policy"`
}

func Defaults() Config {
	return Config{
		Provider: ProviderConfig{
			Default: "codex",
		},
		Retry: RetryConfig{
			MaxRetries: 3,
			Delays:     []string{"0s", "5s", "15s"},
		},
		Providers: ProvidersConfig{
			Codex: GenericProviderConfig{
				Enabled:        true,
				Model:          "default",
				ApprovalPolicy: "on-failure",
				SandboxPolicy:  "workspace-write",
			},
			Claude: GenericProviderConfig{
				Enabled: false,
			},
			Gemini: GenericProviderConfig{
				Enabled: false,
			},
		},
	}
}

func ResolvePath(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}

	if envPath := strings.TrimSpace(os.Getenv("DAEDALUS_CONFIG")); envPath != "" {
		return envPath, nil
	}

	xdgConfigHome := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME"))
	if xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "daedalus", "config.toml"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "daedalus", "config.toml"), nil
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("failed reading config file %s: %w", path, err)
	}

	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed parsing config file %s: %w", path, err)
	}

	applyFallbacks(&cfg)
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Validate(cfg Config) error {
	if strings.TrimSpace(cfg.Provider.Default) == "" {
		return fmt.Errorf("provider.default is required")
	}

	if cfg.Retry.MaxRetries < 0 {
		return fmt.Errorf("retry.max_retries must be >= 0")
	}

	if cfg.Retry.MaxRetries > 0 && len(cfg.Retry.Delays) == 0 {
		return fmt.Errorf("retry.delays must not be empty when retry.max_retries > 0")
	}

	for _, delay := range cfg.Retry.Delays {
		if _, err := time.ParseDuration(delay); err != nil {
			return fmt.Errorf("invalid retry delay %q: %w", delay, err)
		}
	}

	return nil
}

func ParseRetryDelays(delays []string) ([]time.Duration, error) {
	parsed := make([]time.Duration, 0, len(delays))
	for _, delay := range delays {
		value, err := time.ParseDuration(delay)
		if err != nil {
			return nil, fmt.Errorf("invalid retry delay %q: %w", delay, err)
		}
		parsed = append(parsed, value)
	}
	return parsed, nil
}

func applyFallbacks(cfg *Config) {
	defaults := Defaults()

	if strings.TrimSpace(cfg.Provider.Default) == "" {
		cfg.Provider.Default = defaults.Provider.Default
	}

	if cfg.Retry.MaxRetries == 0 && len(cfg.Retry.Delays) == 0 {
		cfg.Retry = defaults.Retry
	}

	if len(cfg.Retry.Delays) == 0 {
		cfg.Retry.Delays = defaults.Retry.Delays
	}

	if cfg.Providers.Codex.Model == "" {
		cfg.Providers.Codex.Model = defaults.Providers.Codex.Model
	}
	if cfg.Providers.Codex.ApprovalPolicy == "" {
		cfg.Providers.Codex.ApprovalPolicy = defaults.Providers.Codex.ApprovalPolicy
	}
	if cfg.Providers.Codex.SandboxPolicy == "" {
		cfg.Providers.Codex.SandboxPolicy = defaults.Providers.Codex.SandboxPolicy
	}
}
