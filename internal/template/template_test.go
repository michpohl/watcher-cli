package template

import (
	"strings"
	"testing"
	"time"
)

func TestExpand(t *testing.T) {
	now := time.Now().Add(-2 * time.Hour)
	ctx := Context{
		Path:    "/tmp/foo/bar.txt",
		RelPath: "foo/bar.txt",
		Event:   "create",
		Size:    123,
		ModTime: now,
		Age:     time.Since(now),
	}
	out := Expand("p={path} dir={dir} name={name} stem={stem} ext={ext} rel={relpath} evt={event} size={size} age={age_ms}", ctx)
	if !strings.Contains(out, "p=/tmp/foo/bar.txt") {
		t.Fatalf("expected path replacement, got %s", out)
	}
	if !strings.Contains(out, "dir=/tmp/foo") {
		t.Fatalf("expected dir replacement")
	}
	if !strings.Contains(out, "name=bar.txt") || !strings.Contains(out, "stem=bar") || !strings.Contains(out, "ext=.txt") {
		t.Fatalf("expected name/stem/ext replacements")
	}
	if !strings.Contains(out, "rel=foo/bar.txt") {
		t.Fatalf("expected relpath replacement")
	}
	if !strings.Contains(out, "evt=create") {
		t.Fatalf("expected event replacement")
	}
	if !strings.Contains(out, "size=123") {
		t.Fatalf("expected size replacement")
	}
}
