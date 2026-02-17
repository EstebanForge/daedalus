package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/EstebanForge/daedalus/internal/config"
	"github.com/EstebanForge/daedalus/internal/loop"
	"github.com/EstebanForge/daedalus/internal/prd"
	"github.com/EstebanForge/daedalus/internal/providers"
)

type App struct {
	version string
}

type globalOptions struct {
	ConfigPath     string
	Provider       string
	ProviderSet    bool
	MaxRetries     int
	MaxRetriesSet  bool
	RetryDelays    []string
	RetryDelaysSet bool
}

type runOptions struct {
	Name string

	Provider       string
	ProviderSet    bool
	MaxRetries     int
	MaxRetriesSet  bool
	RetryDelays    []string
	RetryDelaysSet bool
}

func New(version string) App {
	return App{version: version}
}

func (a App) Run(ctx context.Context, args []string) error {
	global, remainingArgs, err := parseGlobalOptions(args)
	if err != nil {
		return err
	}

	configPath, err := config.ResolvePath(global.ConfigPath)
	if err != nil {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	baseDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to read working directory: %w", err)
	}

	store := prd.NewStore(baseDir)
	command := ""
	if len(remainingArgs) > 0 {
		command = remainingArgs[0]
	}

	switch command {
	case "", "tui":
		return errors.New("TUI is not implemented yet; use 'daedalus run <name>'")
	case "new":
		return a.runNew(store, remainingArgs[1:])
	case "list":
		return a.runList(store)
	case "status":
		return a.runStatus(store, remainingArgs[1:])
	case "validate":
		return a.runValidate(store, remainingArgs[1:])
	case "run":
		return a.runLoop(ctx, store, cfg, global, remainingArgs[1:])
	case "edit":
		return errors.New("'edit' is reserved and not implemented yet")
	case "help", "-h", "--help":
		a.printHelp()
		return nil
	case "version", "-v", "--version":
		fmt.Printf("daedalus version %s\n", a.version)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (a App) runNew(store prd.Store, args []string) error {
	name := "main"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		name = args[0]
	}

	if err := store.Create(name); err != nil {
		return err
	}
	fmt.Printf("Created PRD %q under .daedalus/prds/%s/\n", name, name)
	return nil
}

func (a App) runList(store prd.Store) error {
	summaries, err := store.List()
	if err != nil {
		return err
	}
	if len(summaries) == 0 {
		fmt.Println("No PRDs found.")
		return nil
	}

	for _, summary := range summaries {
		fmt.Printf("%s  %d/%d complete  in-progress:%d\n", summary.Name, summary.Complete, summary.Total, summary.InProgress)
	}
	return nil
}

func (a App) runStatus(store prd.Store, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	name, err := store.ResolveName(name)
	if err != nil {
		return err
	}
	doc, err := store.Load(name)
	if err != nil {
		return err
	}

	fmt.Printf("PRD: %s\n", name)
	fmt.Printf("Project: %s\n", doc.Project)
	fmt.Printf("Stories: %d total\n", len(doc.UserStories))
	fmt.Printf("  complete: %d\n", doc.CountComplete())
	fmt.Printf("  in-progress: %d\n", doc.CountInProgress())
	fmt.Printf("  pending: %d\n", len(doc.UserStories)-doc.CountComplete()-doc.CountInProgress())

	next := doc.NextStory()
	if next == nil {
		fmt.Println("Next: none (all complete)")
		return nil
	}
	fmt.Printf("Next: %s - %s\n", next.ID, next.Title)
	return nil
}

func (a App) runValidate(store prd.Store, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	name, err := store.ResolveName(name)
	if err != nil {
		return err
	}
	doc, err := store.Load(name)
	if err != nil {
		return err
	}

	result := prd.Validate(doc)
	if result.Valid() {
		fmt.Printf("PRD %q is valid.\n", name)
		return nil
	}

	fmt.Printf("PRD %q is invalid:\n", name)
	for _, validationErr := range result.Errors {
		fmt.Printf("- %s\n", validationErr)
	}
	return fmt.Errorf("validation failed")
}

func (a App) runLoop(ctx context.Context, store prd.Store, cfg config.Config, global globalOptions, args []string) error {
	run, err := parseRunOptions(args)
	if err != nil {
		return err
	}

	providerName, maxRetries, retryDelays, err := resolveRuntimeSettings(cfg, global, run)
	if err != nil {
		return err
	}

	name := run.Name
	name, err = store.ResolveName(name)
	if err != nil {
		return err
	}

	registry := providers.NewRegistry()
	provider, err := registry.Resolve(providerName, cfg)
	if err != nil {
		return err
	}

	manager := loop.NewManager(store, provider, loop.RetryPolicy{
		MaxRetries: maxRetries,
		Delays:     retryDelays,
	})
	if err := manager.RunOnce(ctx, name, "."); err != nil {
		return err
	}

	fmt.Printf("Run completed with provider %q.\n", provider.Name())
	return nil
}

func (a App) printHelp() {
	fmt.Println("Daedalus - Codex-native autonomous delivery loop")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  daedalus [--config <path>] [--provider <name>] [--max-retries <n>] [--retry-delays <csv>] [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  new [name]          Create a PRD scaffold")
	fmt.Println("  list                List PRDs")
	fmt.Println("  status [name]       Show PRD status")
	fmt.Println("  validate [name]     Validate PRD JSON")
	fmt.Println("  run [name]          Run one iteration")
	fmt.Println("  edit [name]         Reserved")
	fmt.Println("  help                Show help")
	fmt.Println("  version             Show version")
}

func parseGlobalOptions(args []string) (globalOptions, []string, error) {
	options := globalOptions{}
	index := 0

	for index < len(args) {
		token := args[index]
		if !strings.HasPrefix(token, "--") {
			break
		}
		if token == "--help" || token == "--version" {
			break
		}
		if token == "--" {
			index++
			break
		}

		key, value, hasValue, err := splitFlag(token)
		if err != nil {
			return globalOptions{}, nil, err
		}

		switch key {
		case "config":
			if !hasValue {
				index++
				if index >= len(args) {
					return globalOptions{}, nil, fmt.Errorf("--config requires a value")
				}
				value = args[index]
			}
			options.ConfigPath = strings.TrimSpace(value)
		case "provider":
			if !hasValue {
				index++
				if index >= len(args) {
					return globalOptions{}, nil, fmt.Errorf("--provider requires a value")
				}
				value = args[index]
			}
			options.Provider = strings.TrimSpace(value)
			options.ProviderSet = true
		case "max-retries":
			if !hasValue {
				index++
				if index >= len(args) {
					return globalOptions{}, nil, fmt.Errorf("--max-retries requires a value")
				}
				value = args[index]
			}
			retries, parseErr := strconv.Atoi(value)
			if parseErr != nil || retries < 0 {
				return globalOptions{}, nil, fmt.Errorf("--max-retries must be a non-negative integer")
			}
			options.MaxRetries = retries
			options.MaxRetriesSet = true
		case "retry-delays":
			if !hasValue {
				index++
				if index >= len(args) {
					return globalOptions{}, nil, fmt.Errorf("--retry-delays requires a value")
				}
				value = args[index]
			}
			options.RetryDelays = parseCSV(value)
			options.RetryDelaysSet = true
		default:
			return globalOptions{}, nil, fmt.Errorf("unknown global flag: --%s", key)
		}
		index++
	}

	return options, args[index:], nil
}

func parseRunOptions(args []string) (runOptions, error) {
	options := runOptions{}

	for i := 0; i < len(args); i++ {
		token := args[i]
		if !strings.HasPrefix(token, "--") {
			if options.Name == "" {
				options.Name = token
				continue
			}
			return runOptions{}, fmt.Errorf("unexpected argument: %s", token)
		}

		key, value, hasValue, err := splitFlag(token)
		if err != nil {
			return runOptions{}, err
		}

		switch key {
		case "provider":
			if !hasValue {
				i++
				if i >= len(args) {
					return runOptions{}, fmt.Errorf("--provider requires a value")
				}
				value = args[i]
			}
			options.Provider = strings.TrimSpace(value)
			options.ProviderSet = true
		case "max-retries":
			if !hasValue {
				i++
				if i >= len(args) {
					return runOptions{}, fmt.Errorf("--max-retries requires a value")
				}
				value = args[i]
			}
			retries, parseErr := strconv.Atoi(value)
			if parseErr != nil || retries < 0 {
				return runOptions{}, fmt.Errorf("--max-retries must be a non-negative integer")
			}
			options.MaxRetries = retries
			options.MaxRetriesSet = true
		case "retry-delays":
			if !hasValue {
				i++
				if i >= len(args) {
					return runOptions{}, fmt.Errorf("--retry-delays requires a value")
				}
				value = args[i]
			}
			options.RetryDelays = parseCSV(value)
			options.RetryDelaysSet = true
		default:
			return runOptions{}, fmt.Errorf("unknown run flag: --%s", key)
		}
	}

	return options, nil
}

func splitFlag(token string) (key, value string, hasValue bool, err error) {
	if !strings.HasPrefix(token, "--") {
		return "", "", false, fmt.Errorf("invalid flag token: %s", token)
	}
	body := strings.TrimPrefix(token, "--")
	if body == "" {
		return "", "", false, fmt.Errorf("invalid flag token: %s", token)
	}

	parts := strings.SplitN(body, "=", 2)
	key = strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", false, fmt.Errorf("invalid flag token: %s", token)
	}
	if len(parts) == 2 {
		return key, parts[1], true, nil
	}
	return key, "", false, nil
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func resolveRuntimeSettings(cfg config.Config, global globalOptions, run runOptions) (string, int, []time.Duration, error) {
	providerName := cfg.Provider.Default
	if envProvider := strings.TrimSpace(os.Getenv("DAEDALUS_PROVIDER")); envProvider != "" {
		providerName = envProvider
	}
	if global.ProviderSet {
		providerName = global.Provider
	}
	if run.ProviderSet {
		providerName = run.Provider
	}
	if strings.TrimSpace(providerName) == "" {
		return "", 0, nil, fmt.Errorf("provider is required")
	}

	maxRetries := cfg.Retry.MaxRetries
	if envRetries := strings.TrimSpace(os.Getenv("DAEDALUS_MAX_RETRIES")); envRetries != "" {
		value, err := strconv.Atoi(envRetries)
		if err != nil || value < 0 {
			return "", 0, nil, fmt.Errorf("DAEDALUS_MAX_RETRIES must be a non-negative integer")
		}
		maxRetries = value
	}
	if global.MaxRetriesSet {
		maxRetries = global.MaxRetries
	}
	if run.MaxRetriesSet {
		maxRetries = run.MaxRetries
	}

	retryDelayStrings := cfg.Retry.Delays
	if envDelayCSV := strings.TrimSpace(os.Getenv("DAEDALUS_RETRY_DELAYS")); envDelayCSV != "" {
		retryDelayStrings = parseCSV(envDelayCSV)
	}
	if global.RetryDelaysSet {
		retryDelayStrings = global.RetryDelays
	}
	if run.RetryDelaysSet {
		retryDelayStrings = run.RetryDelays
	}

	if maxRetries > 0 && len(retryDelayStrings) == 0 {
		return "", 0, nil, fmt.Errorf("retry delays must not be empty when max retries is greater than zero")
	}

	retryDelays, err := config.ParseRetryDelays(retryDelayStrings)
	if err != nil {
		return "", 0, nil, err
	}
	return providerName, maxRetries, retryDelays, nil
}
