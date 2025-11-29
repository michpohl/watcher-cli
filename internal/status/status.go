package status

import (
	"sync"
	"time"
)

// Counter aggregates per-action stats.
type Counter struct {
	EventsSeen   int64
	ActionsRun   int64
	ActionsOK    int64
	ActionsError int64
	LastError    string
	LastRun      time.Time
}

// Tracker keeps stats per watch/action.
type Tracker struct {
	mu      sync.Mutex
	Watches map[string]*Counter
}

// NewTracker builds a tracker.
func NewTracker() *Tracker {
	return &Tracker{Watches: map[string]*Counter{}}
}

// IncEvent increments event count.
func (t *Tracker) IncEvent(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.ensure(name)
	c.EventsSeen++
	c.LastRun = time.Now()
}

// IncAction increments action counts.
func (t *Tracker) IncAction(name string, ok bool, errStr string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	c := t.ensure(name)
	c.ActionsRun++
	if ok {
		c.ActionsOK++
	} else {
		c.ActionsError++
		c.LastError = errStr
	}
	c.LastRun = time.Now()
}

// Snapshot returns a copy of stats.
func (t *Tracker) Snapshot() map[string]Counter {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[string]Counter, len(t.Watches))
	for k, v := range t.Watches {
		out[k] = *v
	}
	return out
}

func (t *Tracker) ensure(name string) *Counter {
	if v, ok := t.Watches[name]; ok {
		return v
	}
	c := &Counter{}
	t.Watches[name] = c
	return c
}
