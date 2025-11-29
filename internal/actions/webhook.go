package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"watcher-cli/internal/config"
	"watcher-cli/internal/template"
)

// WebhookRunner posts event payloads.
type WebhookRunner struct {
	Client *http.Client
}

func (r *WebhookRunner) Run(ctx context.Context, ev Context, cfg config.Action) error {
	url := template.Expand(cfg.URL, BuildTemplateContext(ev))
	if url == "" {
		return nil
	}
	payload := map[string]interface{}{
		"path":      ev.Path,
		"relpath":   ev.RelPath,
		"prev_path": ev.PrevPath,
		"event":     ev.Event,
		"size":      ev.Size,
		"mtime":     ev.ModTime,
		"age_ms":    ev.Age.Milliseconds(),
		"is_dir":    ev.IsDir,
	}
	body, _ := json.Marshal(payload)
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}
