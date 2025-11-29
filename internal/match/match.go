package match

import (
	"path/filepath"
	"strings"

	"watcher-cli/internal/config"
	"watcher-cli/internal/scanner"
)

// Matcher applies action filters to events.
type Matcher struct{}

// New returns a matcher.
func New() *Matcher {
	return &Matcher{}
}

// Match returns actions that should run for the event.
func (m *Matcher) Match(ev scanner.Event, watch config.Watch) []config.Action {
	var selected []config.Action
	for _, a := range watch.Actions {
		if !eventAllowed(ev, a) {
			continue
		}
		if !a.MatchesInclude(ev.RelPath) {
			continue
		}
		if a.MatchesExclude(ev.RelPath) {
			continue
		}
		if !conditionsPass(ev, a.Condition) {
			continue
		}
		selected = append(selected, a)
		if watch.StopOnFirstMatch {
			break
		}
	}
	return selected
}

func eventAllowed(ev scanner.Event, a config.Action) bool {
	types := map[config.EventType]struct{}{}
	for _, t := range a.Events {
		types[t] = struct{}{}
	}
	_, ok := types[config.EventType(ev.Type)]
	return ok
}

func conditionsPass(ev scanner.Event, c config.Condition) bool {
	if c.MinSizeBytes > 0 && ev.Info.Size < c.MinSizeBytes {
		return false
	}
	if c.MaxSizeBytes > 0 && ev.Info.Size > c.MaxSizeBytes {
		return false
	}
	if c.MinAge.Duration() > 0 && ev.Age < c.MinAge.Duration() {
		return false
	}
	if c.MaxAge.Duration() > 0 && ev.Age > c.MaxAge.Duration() {
		return false
	}
	if c.OnlyFiles && ev.Info.IsDir {
		return false
	}
	if c.OnlyDirs && !ev.Info.IsDir {
		return false
	}
	if c.IgnoreHidden != nil && *c.IgnoreHidden {
		if isHidden(ev.RelPath) {
			return false
		}
	}
	return true
}

func isHidden(relPath string) bool {
	parts := strings.Split(relPath, string(filepath.Separator))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, ".") {
			return true
		}
	}
	return false
}
