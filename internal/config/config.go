package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"
)

// EventType enumerates filesystem events we handle.
type EventType string

const (
	EventCreate EventType = "create"
	EventModify EventType = "modify"
	EventDelete EventType = "delete"
	EventMove   EventType = "move"
)

// ActionType enumerates supported action kinds.
type ActionType string

const (
	ActionExec    ActionType = "exec"
	ActionCopy    ActionType = "copy"
	ActionMove    ActionType = "move"
	ActionRename  ActionType = "rename"
	ActionWebhook ActionType = "webhook"
)

// Defaults holds global defaults.
type Defaults struct {
	Overwrite bool `yaml:"overwrite"`
}

// Global applies to all watches unless overridden.
type Global struct {
	ScanInterval time.Duration `yaml:"scan_interval_ms"`
	Debounce     time.Duration `yaml:"debounce_ms"`
	DryRun       bool          `yaml:"dry_run"`
	Defaults     Defaults      `yaml:"defaults"`
}

// Condition filters actions.
type Condition struct {
	MinSizeBytes int64         `yaml:"min_size_bytes"`
	MaxSizeBytes int64         `yaml:"max_size_bytes"`
	MinAge       time.Duration `yaml:"min_age_ms"`
	MaxAge       time.Duration `yaml:"max_age_ms"`
	OnlyFiles    bool          `yaml:"only_files"`
	OnlyDirs     bool          `yaml:"only_dirs"`
	IgnoreHidden *bool         `yaml:"ignore_hidden"`
}

// Action describes an action bound to a watch.
type Action struct {
	Name      string            `yaml:"name"`
	Type      ActionType        `yaml:"type"`
	Include   []string          `yaml:"include"`
	Exclude   []string          `yaml:"exclude"`
	Events    []EventType       `yaml:"events"`
	Dest      string            `yaml:"dest"` // copy/move/rename
	Cmd       string            `yaml:"cmd"`  // exec
	URL       string            `yaml:"url"`  // webhook
	Env       map[string]string `yaml:"env"`
	Cwd       string            `yaml:"cwd"`
	Timeout   time.Duration     `yaml:"timeout_ms"`
	Retries   int               `yaml:"retries"`
	Overwrite *bool             `yaml:"overwrite"`
	Condition Condition         `yaml:"condition"`

	compiledIncludes []*doublestar.Glob
	compiledExcludes []*doublestar.Glob
}

// Watch is a folder with actions.
type Watch struct {
	Path             string        `yaml:"path"`
	Recursive        bool          `yaml:"recursive"`
	ScanInterval     time.Duration `yaml:"scan_interval_ms"`
	Debounce         time.Duration `yaml:"debounce_ms"`
	StopOnFirstMatch bool          `yaml:"stop_on_first_match"`
	Actions          []Action      `yaml:"actions"`
}

// Config is the root.
type Config struct {
	Global  Global  `yaml:"global"`
	Watches []Watch `yaml:"watches"`
}

// Load reads and validates the config file.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	expanded := os.ExpandEnv(string(data))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	cfg.normalizeDurations()
	if err := cfg.applyDefaults(); err != nil {
		return Config{}, err
	}
	if err := cfg.compilePatterns(); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate verifies config consistency.
func (c *Config) Validate() error {
	if len(c.Watches) == 0 {
		return errors.New("at least one watch must be defined")
	}
	for i := range c.Watches {
		w := &c.Watches[i]
		if w.Path == "" {
			return fmt.Errorf("watch %d: path is required", i)
		}
		if _, err := os.Stat(w.Path); err != nil {
			return fmt.Errorf("watch %s: path error: %w", w.Path, err)
		}
		if w.ScanInterval <= 0 {
			return fmt.Errorf("watch %s: scan_interval_ms must be > 0", w.Path)
		}
		if w.Debounce < 0 {
			return fmt.Errorf("watch %s: debounce_ms must be >= 0", w.Path)
		}
		if len(w.Actions) == 0 {
			return fmt.Errorf("watch %s: at least one action is required", w.Path)
		}
		names := map[string]struct{}{}
		for j := range w.Actions {
			a := &w.Actions[j]
			if a.Name == "" {
				return fmt.Errorf("watch %s action %d: name required", w.Path, j)
			}
			if _, exists := names[a.Name]; exists {
				return fmt.Errorf("watch %s: duplicate action name %s", w.Path, a.Name)
			}
			names[a.Name] = struct{}{}
			if err := validateAction(a); err != nil {
				return fmt.Errorf("watch %s action %s: %w", w.Path, a.Name, err)
			}
		}
	}
	return nil
}

func validateAction(a *Action) error {
	switch a.Type {
	case ActionExec:
		if strings.TrimSpace(a.Cmd) == "" {
			return errors.New("exec action requires cmd")
		}
	case ActionCopy, ActionMove, ActionRename:
		if strings.TrimSpace(a.Dest) == "" {
			return fmt.Errorf("%s action requires dest", a.Type)
		}
	case ActionWebhook:
		if strings.TrimSpace(a.URL) == "" {
			return errors.New("webhook action requires url")
		}
	default:
		return fmt.Errorf("unknown action type %q", a.Type)
	}
	if len(a.Events) == 0 {
		a.Events = []EventType{EventCreate, EventModify}
	}
	if a.Condition.OnlyDirs && a.Condition.OnlyFiles {
		return errors.New("cannot set both only_dirs and only_files")
	}
	return nil
}

func (c *Config) applyDefaults() error {
	if c.Global.ScanInterval == 0 {
		c.Global.ScanInterval = 1000 * time.Millisecond
	}
	if c.Global.Debounce == 0 {
		c.Global.Debounce = 200 * time.Millisecond
	}
	for i := range c.Watches {
		w := &c.Watches[i]
		if w.ScanInterval == 0 {
			w.ScanInterval = c.Global.ScanInterval
		}
		if w.Debounce == 0 {
			w.Debounce = c.Global.Debounce
		}
		for j := range w.Actions {
			a := &w.Actions[j]
			if a.Timeout == 0 {
				a.Timeout = 30 * time.Second
			}
			if a.Retries < 0 {
				a.Retries = 0
			}
			if a.Overwrite == nil {
				defaultOverwrite := c.Global.Defaults.Overwrite
				a.Overwrite = &defaultOverwrite
			}
			if a.Condition.IgnoreHidden == nil {
				def := true
				a.Condition.IgnoreHidden = &def
			}
		}
	}
	return nil
}

func (c *Config) normalizeDurations() {
	if c.Global.ScanInterval > 0 {
		c.Global.ScanInterval = fromMillis(c.Global.ScanInterval)
	}
	if c.Global.Debounce > 0 {
		c.Global.Debounce = fromMillis(c.Global.Debounce)
	}
	for i := range c.Watches {
		w := &c.Watches[i]
		if w.ScanInterval > 0 {
			w.ScanInterval = fromMillis(w.ScanInterval)
		}
		if w.Debounce > 0 {
			w.Debounce = fromMillis(w.Debounce)
		}
		for j := range w.Actions {
			a := &w.Actions[j]
			if a.Timeout > 0 {
				a.Timeout = fromMillis(a.Timeout)
			}
			if a.Condition.MinAge > 0 {
				a.Condition.MinAge = fromMillis(a.Condition.MinAge)
			}
			if a.Condition.MaxAge > 0 {
				a.Condition.MaxAge = fromMillis(a.Condition.MaxAge)
			}
		}
	}
}

func fromMillis(d time.Duration) time.Duration {
	return time.Duration(int64(d)) * time.Millisecond
}

func (c *Config) compilePatterns() error {
	for i := range c.Watches {
		for j := range c.Watches[i].Actions {
			a := &c.Watches[i].Actions[j]
			incs, err := compilePatterns(a.Include)
			if err != nil {
				return fmt.Errorf("compile include for action %s: %w", a.Name, err)
			}
			excs, err := compilePatterns(a.Exclude)
			if err != nil {
				return fmt.Errorf("compile exclude for action %s: %w", a.Name, err)
			}
			a.compiledIncludes = incs
			a.compiledExcludes = excs
		}
	}
	return nil
}

func compilePatterns(patterns []string) ([]*doublestar.Glob, error) {
	var res []*doublestar.Glob
	for _, p := range patterns {
		g, err := doublestar.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("bad pattern %q: %w", p, err)
		}
		res = append(res, g)
	}
	return res, nil
}

// MatchesInclude tests include patterns; if none, default allow.
func (a *Action) MatchesInclude(relPath string) bool {
	if len(a.Include) > 0 && len(a.compiledIncludes) == 0 {
		if incs, err := compilePatterns(a.Include); err == nil {
			a.compiledIncludes = incs
		}
	}
	if len(a.compiledIncludes) == 0 {
		return true
	}
	for _, g := range a.compiledIncludes {
		if g.Match(relPath) {
			return true
		}
	}
	return false
}

// MatchesExclude tests exclude patterns.
func (a *Action) MatchesExclude(relPath string) bool {
	if len(a.Exclude) > 0 && len(a.compiledExcludes) == 0 {
		if excs, err := compilePatterns(a.Exclude); err == nil {
			a.compiledExcludes = excs
		}
	}
	for _, g := range a.compiledExcludes {
		if g.Match(relPath) {
			return true
		}
	}
	return false
}

// ResolvePaths cleans watch paths.
func (c *Config) ResolvePaths() error {
	for i := range c.Watches {
		p, err := filepath.Abs(c.Watches[i].Path)
		if err != nil {
			return err
		}
		c.Watches[i].Path = p
	}
	return nil
}
