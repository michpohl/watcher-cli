package actions

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"watcher-cli/internal/config"
	"watcher-cli/internal/template"
)

// ExecRunner runs shell commands.
type ExecRunner struct{}

func (r *ExecRunner) Run(ctx context.Context, ev Context, cfg config.Action) error {
	cmdStr := template.Expand(cfg.Cmd, BuildTemplateContext(ev))
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	if cfg.Cwd != "" {
		cmd.Dir = cfg.Cwd
	}
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+template.Expand(v, BuildTemplateContext(ev)))
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
