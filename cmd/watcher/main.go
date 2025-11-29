package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"watcher-cli/internal/actions"
	"watcher-cli/internal/config"
	"watcher-cli/internal/logging"
	"watcher-cli/internal/match"
	"watcher-cli/internal/scanner"
	"watcher-cli/internal/watcher"
)

func main() {
	root := &cobra.Command{
		Use:   "watcher",
		Short: "Directory watcher with per-action filters",
	}

	var cfgPath string
	root.PersistentFlags().StringVar(&cfgPath, "config", "watcher.yaml", "path to config file")

	root.AddCommand(runCmd(&cfgPath))
	root.AddCommand(validateCmd(&cfgPath))
	root.AddCommand(initCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(simulateCmd(&cfgPath))

	if err := root.Execute(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

func runCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start watcher",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			if err := cfg.ResolvePaths(); err != nil {
				return err
			}
			logger := logging.New(slog.LevelInfo)
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			super := watcher.NewSupervisor(cfg, logger, cfg.Global.DryRun)
			logger.Info("starting watcher", "watches", len(cfg.Watches))
			return super.Run(ctx)
		},
	}
}

func validateCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			if err := cfg.ResolvePaths(); err != nil {
				return err
			}
			fmt.Println("config OK")
			return nil
		},
	}
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Write sample config",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "watcher.yaml"
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists", path)
			}
			if err := os.WriteFile(path, []byte(sampleConfig), 0o644); err != nil {
				return err
			}
			fmt.Println("wrote", path)
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status (available while running in same process)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("status is available during run; expose via future IPC")
			return nil
		},
	}
}

func simulateCmd(cfgPath *string) *cobra.Command {
	var watchPath string
	var eventType string
	var filePath string
	var size int64
	var age time.Duration
	var execute bool
	cmd := &cobra.Command{
		Use:   "simulate",
		Short: "Simulate an event through matching/actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return err
			}
			if err := cfg.ResolvePaths(); err != nil {
				return err
			}
			w := pickWatch(cfg.Watches, watchPath)
			if w == nil {
				return fmt.Errorf("watch not found: %s", watchPath)
			}
			if filePath == "" {
				return fmt.Errorf("--file is required")
			}
			if eventType == "" {
				eventType = string(config.EventCreate)
			}
			rel, _ := filepath.Rel(w.Path, filePath)
			info := scanner.FileInfo{
				Size:    size,
				ModTime: time.Now().Add(-age),
				IsDir:   false,
			}
			ev := scanner.Event{
				Path:    filePath,
				RelPath: rel,
				Type:    eventType,
				Info:    info,
				Age:     age,
			}
			m := match.New()
			selected := m.Match(ev, *w)
			if len(selected) == 0 {
				fmt.Println("no actions matched")
				return nil
			}
			exec := &actions.Executor{Registry: actions.NewRegistry(), DryRun: !execute}
			ctx := context.Background()
			for _, a := range selected {
				err := exec.Execute(ctx, actions.Context{
					Path:     ev.Path,
					RelPath:  ev.RelPath,
					PrevPath: ev.PrevPath,
					Event:    ev.Type,
					Size:     ev.Info.Size,
					ModTime:  ev.Info.ModTime,
					Age:      ev.Age,
					IsDir:    ev.Info.IsDir,
				}, a)
				if err != nil {
					fmt.Printf("action %s error: %v\n", a.Name, err)
				} else {
					if execute {
						fmt.Printf("action %s executed\n", a.Name)
					} else {
						fmt.Printf("action %s (dry-run)\n", a.Name)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&watchPath, "watch", "", "watch path to use (defaults to first)")
	cmd.Flags().StringVar(&eventType, "event", "create", "event type (create|modify|delete|move)")
	cmd.Flags().StringVar(&filePath, "file", "", "file path for the simulated event")
	cmd.Flags().Int64Var(&size, "size", 0, "file size bytes")
	cmd.Flags().DurationVar(&age, "age", 0, "age of file (e.g., 10s, 2m)")
	cmd.Flags().BoolVar(&execute, "execute", false, "actually run actions (default dry-run)")
	return cmd
}

func pickWatch(watches []config.Watch, path string) *config.Watch {
	if len(watches) == 0 {
		return nil
	}
	if path == "" {
		return &watches[0]
	}
	for i := range watches {
		if filepath.Clean(watches[i].Path) == filepath.Clean(path) {
			return &watches[i]
		}
	}
	return nil
}

const sampleConfig = `global:
  scan_interval_ms: 1000
  debounce_ms: 200
  dry_run: false
  defaults:
    overwrite: false

watches:
  - path: ./incoming
    recursive: true
    scan_interval_ms: 500
    stop_on_first_match: false
    actions:
      - name: images
        include: ["**/*.jpg", "**/*.png"]
        events: [create, modify]
        type: exec
        cmd: "python process_image.py {path}"
        retries: 3
        timeout_ms: 10000
      - name: pdf_backup
        include: ["**/*.pdf"]
        exclude: ["**/tmp/**"]
        events: [create]
        type: copy
        dest: "/backup/docs/{relpath}"
        overwrite: true
      - name: archive_old
        include: ["**/*"]
        events: [modify]
        type: move
        dest: "{dir}/archive/{name}"
`
