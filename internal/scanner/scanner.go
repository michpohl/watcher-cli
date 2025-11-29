package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo captures file metadata relevant for diffing.
type FileInfo struct {
	Size    int64
	ModTime time.Time
	IsDir   bool
	Mode    fs.FileMode
}

// Snapshot maps absolute paths to file info.
type Snapshot map[string]FileInfo

// Event represents a change detected between snapshots.
type Event struct {
	Path    string
	RelPath string
	PrevPath string
	Type    string // create, modify, delete, move
	Info    FileInfo
	Age     time.Duration
}

// Scanner walks a root directory to produce a snapshot.
type Scanner struct {
	root      string
	recursive bool
}

// New creates a scanner for a root.
func New(root string, recursive bool) *Scanner {
	return &Scanner{root: root, recursive: recursive}
}

// Scan walks the root and builds a snapshot.
func (s *Scanner) Scan() (Snapshot, error) {
	out := make(Snapshot)
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == s.root {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		if !s.recursive && strings.Contains(rel, string(os.PathSeparator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out[path] = FileInfo{
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
			Mode:    info.Mode(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Diff compares previous and current snapshots.
func Diff(root string, prev, curr Snapshot) []Event {
	events := []Event{}
	prevBySignature := map[string]string{}
	deletes := map[string]Event{}
	modifies := []Event{}
	creates := []Event{}
	moves := []Event{}

	for p, info := range prev {
		sig := signature(info)
		prevBySignature[sig] = p
		if _, exists := curr[p]; !exists {
			deletes[p] = Event{
				Path:    p,
				RelPath: rel(root, p),
				Type:    "delete",
				Info:    info,
			}
		}
	}
	for p, info := range curr {
		if prevInfo, exists := prev[p]; exists {
			if hasChanged(prevInfo, info) {
				modifies = append(modifies, Event{
					Path:    p,
					RelPath: rel(root, p),
					Type:    "modify",
					Info:    info,
					Age:     age(info),
				})
			}
			continue
		}
		sig := signature(info)
		if oldPath, ok := prevBySignature[sig]; ok && oldPath != p {
			moves = append(moves, Event{
				Path:     p,
				PrevPath: oldPath,
				RelPath:  rel(root, p),
				Type:     "move",
				Info:     info,
				Age:      age(info),
			})
			delete(deletes, oldPath)
		} else {
			creates = append(creates, Event{
				Path:    p,
				RelPath: rel(root, p),
				Type:    "create",
				Info:    info,
				Age:     age(info),
			})
		}
	}
	for _, ev := range modifies {
		events = append(events, ev)
	}
	for _, ev := range moves {
		events = append(events, ev)
	}
	for _, ev := range creates {
		events = append(events, ev)
	}
	for _, ev := range deletes {
		events = append(events, ev)
	}
	return events
}

func hasChanged(prev, curr FileInfo) bool {
	return prev.Size != curr.Size || !prev.ModTime.Equal(curr.ModTime) || prev.Mode != curr.Mode
}

func signature(info FileInfo) string {
	return fmt.Sprintf("%d-%d-%o", info.Size, info.ModTime.UnixNano(), info.Mode)
}

func rel(root, path string) string {
	r, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return r
}

func age(info FileInfo) time.Duration {
	if info.ModTime.IsZero() {
		return 0
	}
	return time.Since(info.ModTime)
}

// FilterHidden determines if a path is hidden based on components.
func FilterHidden(root, path string) (bool, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false, err
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if strings.HasPrefix(part, ".") {
			return true, nil
		}
	}
	return false, nil
}

// EnsureDir ensures root exists.
func EnsureDir(path string) error {
	if path == "" {
		return errors.New("empty path")
	}
	return os.MkdirAll(path, 0o755)
}
