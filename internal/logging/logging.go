package logging

import (
	"log/slog"
	"os"
)

// New creates a baseline structured logger.
func New(level slog.Leveler) *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
