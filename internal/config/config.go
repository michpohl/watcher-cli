package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	ScanInterval MillisDuration `yaml:"scan_interval_ms"`
	Debounce     MillisDuration `yaml:"debounce_ms"`
	DryRun       bool           `yaml:"dry_run"`
	Defaults     Defaults       `yaml:"defaults"`
}

// Condition filters actions.
type Condition struct {
	MinSizeBytes int64          `yaml:"min_size_bytes"`
	MaxSizeBytes int64          `yaml:"max_size_bytes"`
	MinAge       MillisDuration `yaml:"min_age_ms"`
	MaxAge       MillisDuration `yaml:"max_age_ms"`
	OnlyFiles    bool           `yaml:"only_files"`
	OnlyDirs     bool           `yaml:"only_dirs"`
	IgnoreHidden *bool          `yaml:"ignore_hidden"`
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
	Timeout   MillisDuration    `yaml:"timeout_ms"`
	Retries   int               `yaml:"retries"`
	Overwrite *bool             `yaml:"overwrite"`
	Condition Condition         `yaml:"condition"`
}

// Watch is a folder with actions.
type Watch struct {
	Path             string         `yaml:"path"`
	Recursive        bool           `yaml:"recursive"`
	ScanInterval     MillisDuration `yaml:"scan_interval_ms"`
	Debounce         MillisDuration `yaml:"debounce_ms"`
	StopOnFirstMatch bool           `yaml:"stop_on_first_match"`
	Actions          []Action       `yaml:"actions"`
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
		if w.ScanInterval.Duration() <= 0 {
			return fmt.Errorf("watch %s: scan_interval_ms must be > 0", w.Path)
		}
		if w.Debounce.Duration() < 0 {
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
	if c.Global.ScanInterval.Duration() == 0 {
		c.Global.ScanInterval = MillisFromDuration(1000 * time.Millisecond)
	}
	if c.Global.Debounce.Duration() == 0 {
		c.Global.Debounce = MillisFromDuration(200 * time.Millisecond)
	}
	for i := range c.Watches {
		w := &c.Watches[i]
		if w.ScanInterval.Duration() == 0 {
			w.ScanInterval = c.Global.ScanInterval
		}
		if w.Debounce.Duration() == 0 {
			w.Debounce = c.Global.Debounce
		}
		for j := range w.Actions {
			a := &w.Actions[j]
			if a.Timeout.Duration() == 0 {
				a.Timeout = MillisFromDuration(30 * time.Second)
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
	// no-op now; handled in unmarshal
}

// MillisDuration unmarshals from int (ms) or duration string.
// MillisDuration stores a duration in milliseconds parsed from int or duration string.
type MillisDuration int64

// MillisFromDuration converts time.Duration to MillisDuration.
func MillisFromDuration(d time.Duration) MillisDuration {
	return MillisDuration(d / time.Millisecond)
}

// Duration returns the time.Duration value.
func (d MillisDuration) Duration() time.Duration {
	return time.Duration(int64(d)) * time.Millisecond
}

// UnmarshalYAML implements yaml unmarshalling with ms support.
func (d *MillisDuration) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// try int (milliseconds)
		if v, err := strconv.ParseInt(value.Value, 10, 64); err == nil {
			*d = MillisDuration(v)
			return nil
		}
		// fallback to duration string
		parsed, err := time.ParseDuration(value.Value)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", value.Value, err)
		}
		*d = MillisDuration(parsed.Milliseconds())
		return nil
	default:
		return fmt.Errorf("invalid duration node kind: %v", value.Kind)
	}
}

// MatchesInclude tests include patterns; if none, default allow.
func (a *Action) MatchesInclude(relPath string) bool {
	if len(a.Include) == 0 {
		return true
	}
	p := filepath.ToSlash(relPath)
	for _, pattern := range a.Include {
		if ok, _ := doublestar.PathMatch(pattern, p); ok {
			return true
		}
	}
	return false
}

// MatchesExclude tests exclude patterns.
func (a *Action) MatchesExclude(relPath string) bool {
	if len(a.Exclude) == 0 {
		return false
	}
	p := filepath.ToSlash(relPath)
	for _, pattern := range a.Exclude {
		if ok, _ := doublestar.PathMatch(pattern, p); ok {
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
