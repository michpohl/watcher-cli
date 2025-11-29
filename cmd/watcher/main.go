package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"watcher-cli/internal/config"
	"watcher-cli/internal/logging"
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
			logger := logging.New(0)
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
