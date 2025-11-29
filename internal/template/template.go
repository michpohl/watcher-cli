package template

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Context provides values for token substitution.
type Context struct {
	Path    string
	RelPath string
	Event   string
	Size    int64
	ModTime time.Time
	Age     time.Duration
}

// Expand replaces known tokens in the input string.
func Expand(in string, ctx Context) string {
	// Precompute common fields.
	dir := filepath.Dir(ctx.Path)
	name := filepath.Base(ctx.Path)
	stem := name
	ext := ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		stem = name[:dot]
		ext = name[dot:]
	}
	repl := map[string]string{
		"{path}":     ctx.Path,
		"{relpath}":  ctx.RelPath,
		"{event}":    ctx.Event,
		"{size}":     intToString(ctx.Size),
		"{mtime}":    ctx.ModTime.Format(time.RFC3339),
		"{age_ms}":   intToString(ctx.Age.Milliseconds()),
		"{age_days}": intToString(int64(ctx.Age.Hours() / 24)),
		"{dir}":      dir,
		"{name}":     name,
		"{stem}":     stem,
		"{ext}":      ext,
	}
	out := in
	for k, v := range repl {
		out = strings.ReplaceAll(out, k, v)
	}
	return out
}

func intToString(v int64) string {
	return strconv.FormatInt(v, 10)
}
