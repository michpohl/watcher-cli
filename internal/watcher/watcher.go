package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"watcher-cli/internal/actions"
	"watcher-cli/internal/config"
	"watcher-cli/internal/match"
	"watcher-cli/internal/scanner"
	"watcher-cli/internal/status"
)

// Supervisor manages watch workers.
type Supervisor struct {
	cfg      config.Config
	logger   *slog.Logger
	tracker  *status.Tracker
	executor *actions.Executor
	matcher  *match.Matcher
}

// NewSupervisor constructs a supervisor.
func NewSupervisor(cfg config.Config, logger *slog.Logger, dryRun bool) *Supervisor {
	reg := actions.NewRegistry()
	return &Supervisor{
		cfg:      cfg,
		logger:   logger,
		tracker:  status.NewTracker(),
		executor: &actions.Executor{Registry: reg, DryRun: dryRun},
		matcher:  match.New(),
	}
}

// Run starts all workers and blocks until ctx done.
func (s *Supervisor) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	for _, wcfg := range s.cfg.Watches {
		wg.Add(1)
		go func(w config.Watch) {
			defer wg.Done()
			worker := &Worker{
				cfg:      w,
				logger:   s.logger,
				tracker:  s.tracker,
				executor: s.executor,
				matcher:  s.matcher,
			}
			worker.Run(ctx)
		}(wcfg)
	}
	wg.Wait()
	return nil
}

// Status returns snapshot.
func (s *Supervisor) Status() map[string]status.Counter {
	return s.tracker.Snapshot()
}

// Worker watches a single directory.
type Worker struct {
	cfg      config.Watch
	logger   *slog.Logger
	tracker  *status.Tracker
	executor *actions.Executor
	matcher  *match.Matcher

	prev        snapshotState
	debounceMap map[string]time.Time
}

type snapshotState struct {
	data scanner.Snapshot
}

// Run starts the polling loop.
func (w *Worker) Run(ctx context.Context) {
	scn := scanner.New(w.cfg.Path, w.cfg.Recursive)
	ticker := time.NewTicker(w.cfg.ScanInterval)
	defer ticker.Stop()

	// initial scan
	w.prev.data, _ = scn.Scan()
	w.debounceMap = make(map[string]time.Time)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			curr, err := scn.Scan()
			if err != nil {
				w.logger.Error("scan error", "path", w.cfg.Path, "err", err)
				continue
			}
			events := scanner.Diff(w.cfg.Path, w.prev.data, curr)
			w.prev.data = curr
			for _, ev := range events {
				w.handleEvent(ctx, ev)
			}
		}
	}
}

func (w *Worker) handleEvent(ctx context.Context, ev scanner.Event) {
	if w.cfg.Debounce > 0 {
		last, ok := w.debounceMap[ev.Path]
		if ok && time.Since(last) < w.cfg.Debounce {
			return
		}
		w.debounceMap[ev.Path] = time.Now()
	}
	if ev.Type == "delete" {
		delete(w.debounceMap, ev.Path)
	}
	w.tracker.IncEvent(w.cfg.Path)
	selected := w.matcher.Match(ev, w.cfg)
	for _, action := range selected {
		evCtx := actions.Context{
			Path:     ev.Path,
			RelPath:  ev.RelPath,
			PrevPath: ev.PrevPath,
			Event:    ev.Type,
			Size:     ev.Info.Size,
			ModTime:  ev.Info.ModTime,
			Age:      ev.Age,
			IsDir:    ev.Info.IsDir,
		}
		if w.executor.DryRun {
			w.logger.Info("dry-run action", "watch", w.cfg.Path, "action", action.Name, "event", ev.Type, "path", ev.Path)
			w.tracker.IncAction(w.cfg.Path+"."+action.Name, true, "")
			continue
		}
		err := w.executor.Execute(ctx, evCtx, action)
		if err != nil {
			w.logger.Error("action error", "watch", w.cfg.Path, "action", action.Name, "err", err)
			w.tracker.IncAction(w.cfg.Path+"."+action.Name, false, err.Error())
		} else {
			w.logger.Info("action ok", "watch", w.cfg.Path, "action", action.Name, "event", ev.Type, "path", ev.Path)
			w.tracker.IncAction(w.cfg.Path+"."+action.Name, true, "")
		}
	}
}
