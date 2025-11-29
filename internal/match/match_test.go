package match

import (
	"testing"
	"time"

	"watcher-cli/internal/config"
	"watcher-cli/internal/scanner"
)

func TestMatchIncludesAndExcludes(t *testing.T) {
	m := New()
	w := config.Watch{
		Path: "/tmp",
		Actions: []config.Action{
			{
				Name:    "imgs",
				Type:    config.ActionExec,
				Include: []string{"**/*.jpg"},
				Events:  []config.EventType{config.EventCreate},
			},
			{
				Name:    "docs",
				Type:    config.ActionExec,
				Include: []string{"**/*.pdf"},
				Exclude: []string{"**/tmp/**"},
				Events:  []config.EventType{config.EventCreate},
			},
		},
	}
	_ = w // silence lint if needed
	ev := scanner.Event{
		Path:    "/tmp/a/b/photo.jpg",
		RelPath: "a/b/photo.jpg",
		Type:    "create",
		Info:    scanner.FileInfo{Size: 10},
	}
	actions := m.Match(ev, w)
	if len(actions) != 1 || actions[0].Name != "imgs" {
		t.Fatalf("expected imgs action, got %#v", actions)
	}

	ev2 := scanner.Event{
		Path:    "/tmp/tmp/doc.pdf",
		RelPath: "tmp/doc.pdf",
		Type:    "create",
		Info:    scanner.FileInfo{Size: 10},
	}
	actions = m.Match(ev2, w)
	if len(actions) != 0 {
		t.Fatalf("expected exclude to block, got %#v", actions)
	}

	ev3 := scanner.Event{
		Path:    "/tmp/doc.pdf",
		RelPath: "doc.pdf",
		Type:    "create",
		Info:    scanner.FileInfo{Size: 10},
	}
	actions = m.Match(ev3, w)
	if len(actions) != 1 || actions[0].Name != "docs" {
		t.Fatalf("expected docs action, got %#v", actions)
	}
}

func TestMatchConditions(t *testing.T) {
	m := New()
	minSize := int64(100)
	age := 2 * time.Hour
	w := config.Watch{
		Path: "/tmp",
		Actions: []config.Action{
			{
				Name: "big-old",
				Type: config.ActionExec,
				Events: []config.EventType{
					config.EventModify,
				},
				Condition: config.Condition{
					MinSizeBytes: minSize,
					MinAge:       config.MillisFromDuration(age),
					OnlyFiles:    true,
				},
			},
		},
	}

	ev := scanner.Event{
		Path:    "/tmp/file.bin",
		RelPath: "file.bin",
		Type:    "modify",
		Info:    scanner.FileInfo{Size: 50, IsDir: false},
		Age:     age + time.Minute,
	}
	if got := len(m.Match(ev, w)); got != 0 {
		t.Fatalf("expected size filter to block, got %d", got)
	}

	ev.Info.Size = 200
	ev.Age = age + time.Minute
	if got := len(m.Match(ev, w)); got != 1 {
		t.Fatalf("expected pass after size ok, got %d", got)
	}

	ev.Info.IsDir = true
	if got := len(m.Match(ev, w)); got != 0 {
		t.Fatalf("expected dir to be blocked by OnlyFiles, got %d", got)
	}
}
