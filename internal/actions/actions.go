package actions

import (
	"context"
	"errors"
	"fmt"
	"time"

	"watcher-cli/internal/config"
	"watcher-cli/internal/template"
)

// Runner executes a single action.
type Runner interface {
	Run(ctx context.Context, ev Context, cfg config.Action) error
}

// Registry maps action types to runners.
type Registry struct {
	entries map[config.ActionType]Runner
}

// NewRegistry builds default registry.
func NewRegistry() *Registry {
	r := &Registry{entries: map[config.ActionType]Runner{}}
	r.Register(config.ActionExec, &ExecRunner{})
	r.Register(config.ActionCopy, &CopyMoveRunner{Mode: config.ActionCopy})
	r.Register(config.ActionMove, &CopyMoveRunner{Mode: config.ActionMove})
	r.Register(config.ActionRename, &CopyMoveRunner{Mode: config.ActionRename})
	r.Register(config.ActionWebhook, &WebhookRunner{})
	return r
}

// Register adds a runner.
func (r *Registry) Register(kind config.ActionType, runner Runner) {
	r.entries[kind] = runner
}

// Get fetches a runner.
func (r *Registry) Get(kind config.ActionType) (Runner, bool) {
	val, ok := r.entries[kind]
	return val, ok
}

// Executor wraps runners with retries/timeouts and templating.
type Executor struct {
	Registry *Registry
	DryRun   bool
}

// Context is the data for templating and payloads.
type Context struct {
	Path    string
	RelPath string
	PrevPath string
	Event   string
	Size    int64
	ModTime time.Time
	Age     time.Duration
	IsDir   bool
}

// Execute runs an action with retries and timeout.
func (e *Executor) Execute(ctx context.Context, ev Context, action config.Action) error {
	runner, ok := e.Registry.Get(action.Type)
	if !ok {
		return fmt.Errorf("no runner for type %s", action.Type)
	}
	if e.DryRun {
		return nil
	}
	timeout := action.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	run := func() error {
		ctxRun, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return runner.Run(ctxRun, ev, action)
	}
	var lastErr error
	for attempt := 0; attempt <= action.Retries; attempt++ {
		if err := run(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("unknown action error")
	}
	return lastErr
}

// BuildTemplateContext converts action Context to template.Context.
func BuildTemplateContext(ev Context) template.Context {
	return template.Context{
		Path:    ev.Path,
		RelPath: ev.RelPath,
		Event:   ev.Event,
		Size:    ev.Size,
		ModTime: ev.ModTime,
		Age:     ev.Age,
	}
}
