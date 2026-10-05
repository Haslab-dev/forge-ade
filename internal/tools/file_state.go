package tools

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// FileStateTracker mirrors the harness readFileState concept: it records which
// files the session has Read (mtime + size at read time) so edit/write can
// enforce read-before-edit and staleness checks. One tracker lives per agent
// session (in memory); tools only see it through the context, so standalone
// callers (tests, side chat) are never gated.
type FileStateTracker struct {
	mu    sync.Mutex
	reads map[string]fileStamp
}

type fileStamp struct {
	mtime time.Time
	size  int64
}

func NewFileStateTracker() *FileStateTracker {
	return &FileStateTracker{reads: make(map[string]fileStamp)}
}

// MarkRead records the file's current mtime/size as the read snapshot.
func (t *FileStateTracker) MarkRead(path string) {
	if t == nil {
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	t.mu.Lock()
	t.reads[path] = fileStamp{mtime: fi.ModTime(), size: fi.Size()}
	t.mu.Unlock()
}

// MarkEdited refreshes the snapshot after a successful edit/write so the model
// does not need to re-Read the file it just changed.
func (t *FileStateTracker) MarkEdited(path string) {
	t.MarkRead(path)
}

// CheckFresh returns an error when the file has not been Read yet or changed
// on disk since the last Read (by another process, a linter, the user…).
func (t *FileStateTracker) CheckFresh(path string) error {
	if t == nil {
		return nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("file does not exist: %s", path)
	}
	t.mu.Lock()
	stamp, ok := t.reads[path]
	t.mu.Unlock()
	if !ok {
		return fmt.Errorf("File has not been read yet. Read it first before writing to it.")
	}
	if !fi.ModTime().Equal(stamp.mtime) || fi.Size() != stamp.size {
		return fmt.Errorf("File has been modified since read, either by the user or by a linter. Read it again before attempting to write it.")
	}
	return nil
}

type fileStateKey int

const fileStateCtxKey fileStateKey = iota + 100

// WithFileState attaches a session's read-state tracker to the tool context.
func WithFileState(ctx context.Context, t *FileStateTracker) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, fileStateCtxKey, t)
}

// FileStateFrom returns the tracker attached to the context, if any.
func FileStateFrom(ctx context.Context) *FileStateTracker {
	if ctx == nil {
		return nil
	}
	t, _ := ctx.Value(fileStateCtxKey).(*FileStateTracker)
	return t
}
