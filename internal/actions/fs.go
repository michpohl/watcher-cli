package actions

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"watcher-cli/internal/config"
	"watcher-cli/internal/template"
)

// CopyMoveRunner handles copy/move/rename operations.
type CopyMoveRunner struct {
	Mode config.ActionType
}

func (r *CopyMoveRunner) Run(ctx context.Context, ev Context, cfg config.Action) error {
	destTmpl := template.Expand(cfg.Dest, BuildTemplateContext(ev))
	if destTmpl == "" {
		return fmt.Errorf("empty dest")
	}
	dest := destTmpl
	if cfg.Type == config.ActionRename && ev.RelPath != "" {
		dest = filepath.Join(filepath.Dir(ev.Path), destTmpl)
	}
	overwrite := false
	if cfg.Overwrite != nil {
		overwrite = *cfg.Overwrite
	}
	switch r.Mode {
	case config.ActionCopy:
		return copyFile(ev.Path, dest, overwrite)
	case config.ActionMove, config.ActionRename:
		return moveFile(ev.Path, dest, overwrite)
	default:
		return fmt.Errorf("unsupported mode %s", r.Mode)
	}
}

func copyFile(src, dest string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("dest exists: %s", dest)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

func moveFile(src, dest string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("dest exists: %s", dest)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	// Fallback to copy+remove
	if err := copyFile(src, dest, overwrite); err != nil {
		return err
	}
	return os.Remove(src)
}
