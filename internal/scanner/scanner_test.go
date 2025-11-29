package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiffCreateModifyMoveDelete(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path1, []byte("hi"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	scn := New(dir, true)
	prev, err := scn.Scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	// Modify
	time.Sleep(5 * time.Millisecond)
	if err := os.WriteFile(path1, []byte("hello"), 0o644); err != nil {
		t.Fatalf("modify: %v", err)
	}
	curr, _ := scn.Scan()
	evs := Diff(dir, prev, curr)
	if len(evs) != 1 || evs[0].Type != "modify" {
		t.Fatalf("expected modify event, got %#v", evs)
	}

	// Move
	prev = curr
	path2 := filepath.Join(dir, "moved.txt")
	if err := os.Rename(path1, path2); err != nil {
		t.Fatalf("rename: %v", err)
	}
	curr, _ = scn.Scan()
	evs = Diff(dir, prev, curr)
	if len(evs) != 1 || evs[0].Type != "move" || evs[0].PrevPath != path1 || evs[0].Path != path2 {
		t.Fatalf("expected move event, got %#v", evs)
	}

	// Delete
	prev = curr
	if err := os.Remove(path2); err != nil {
		t.Fatalf("remove: %v", err)
	}
	curr, _ = scn.Scan()
	evs = Diff(dir, prev, curr)
	if len(evs) != 1 || evs[0].Type != "delete" {
		t.Fatalf("expected delete event, got %#v", evs)
	}
}
