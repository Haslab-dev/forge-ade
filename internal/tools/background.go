package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Background task registry — the long-running-task layer (port of ZCode's
// background bash + TaskOutput/TaskStop machinery). `bash
// run_in_background:true` spawns a detached process, registers it here, and
// returns immediately; `task_output` polls or blocks on it; `task_stop` kills
// it. Completed tasks queue a <task-notification> that the agent drains into
// the next model step so the model learns the outcome without polling.
// ---------------------------------------------------------------------------

// TaskStatus is the lifecycle state of a background task.
type TaskStatus string

const (
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed" // exit code 0
	TaskFailed    TaskStatus = "failed"    // non-zero exit
	TaskStopped   TaskStatus = "stopped"   // killed via task_stop
)

// BackgroundTask is one detached shell command.
type BackgroundTask struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	Command   string     `json:"command"`
	Cwd       string     `json:"cwd,omitempty"`
	Status    TaskStatus `json:"status"`
	ExitCode  int        `json:"exit_code,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	Finished  *time.Time `json:"finished_at,omitempty"`

	// OutputFile persists the full stdout+stderr stream so output survives
	// beyond the in-memory ring buffer (ZCode's artifact spill).
	OutputFile string `json:"output_file,omitempty"`
	OutputTail string `json:"output_tail,omitempty"`
	OutputSize int64  `json:"output_size"`

	notified bool // completion notification already drained into the conversation

	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
	tail   *tailWriter
}

// tailWriter tees output to the full log file while keeping a bounded
// in-memory tail for quick task_output reads.
type tailWriter struct {
	mu   sync.Mutex
	file *os.File
	buf  bytes.Buffer
	max  int
}

func (w *tailWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.file.Write(p); err != nil {
		return 0, err
	}
	w.buf.Write(p)
	if w.buf.Len() > w.max {
		drop := w.buf.Len() - w.max
		w.buf.Next(drop)
	}
	return len(p), nil
}

func (w *tailWriter) tail() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

const (
	// outputTailLimit caps the in-memory tail returned by task_output before
	// it points at the full output file.
	outputTailLimit = 16 * 1024
	// maxWaitPollInterval is the poll granularity while blocking in task_output.
	waitPollInterval = 100 * time.Millisecond
)

// taskUpdateFunc is invoked on every task lifecycle transition so the agent
// layer can stream background-task events to the UI.
var taskUpdateFunc func(task *BackgroundTask)

// SetTaskUpdateHandler installs the lifecycle callback (called once from the
// agent manager at construction).
func SetTaskUpdateHandler(fn func(task *BackgroundTask)) {
	taskUpdateFunc = fn
}

// backgroundRegistry is process-global (mirrors the read cache precedent —
// tasks must outlive individual tool handlers and agent turns).
var backgroundRegistry = struct {
	mu    sync.Mutex
	tasks map[string]*BackgroundTask
}{
	tasks: make(map[string]*BackgroundTask),
}

// StartBackgroundCommand spawns a detached shell command and registers it as a
// background task owned by the given agent session. The process is detached
// from the caller's context on purpose: it must survive turn cancellation.
func StartBackgroundCommand(sessionID, command, cwd string) (*BackgroundTask, error) {
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if cwd == "" {
		cwd = defaultCwd(context.Background())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "/bin/zsh", "-l", "-c", command)
	cmd.Dir = cwd
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own process group so StopTask kills children too

	outputFile := filepath.Join(os.TempDir(), fmt.Sprintf("forge-ade-task-%s.log", uuid.New().String()[:8]))
	outFile, err := os.Create(outputFile)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create task output file: %w", err)
	}
	tail := &tailWriter{file: outFile, max: outputTailLimit}
	cmd.Stdout = tail
	cmd.Stderr = tail

	task := &BackgroundTask{
		ID:         uuid.New().String()[:8],
		SessionID:  sessionID,
		Command:    command,
		Cwd:        cwd,
		Status:     TaskRunning,
		StartedAt:  time.Now(),
		OutputFile: outputFile,
		tail:       tail,
		done:       make(chan struct{}),
		cmd:        cmd,
		cancel:     cancel,
	}

	if err := cmd.Start(); err != nil {
		outFile.Close()
		cancel()
		return nil, fmt.Errorf("start command: %w", err)
	}

	backgroundRegistry.mu.Lock()
	backgroundRegistry.tasks[task.ID] = task
	backgroundRegistry.mu.Unlock()
	notifyTaskUpdate(task)

	go func() {
		defer outFile.Close()
		err := cmd.Wait()

		now := time.Now()
		task.Finished = &now
		if ctx.Err() != nil {
			task.Status = TaskStopped
			task.ExitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			task.Status = TaskFailed
			task.ExitCode = exitErr.ExitCode()
		} else if err != nil {
			task.Status = TaskFailed
			task.ExitCode = 1
		} else {
			task.Status = TaskCompleted
			task.ExitCode = 0
		}
		task.OutputTail = task.tail.tail()
		task.OutputSize = fileSizeOrZero(outputFile)
		close(task.done)
		notifyTaskUpdate(task)
	}()

	return task, nil
}

// tailWriter is wired as a live tap on the output stream. We use a simple
// approach instead: read the tail from the output file on demand.
func fileSizeOrZero(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func notifyTaskUpdate(task *BackgroundTask) {
	if taskUpdateFunc != nil {
		taskUpdateFunc(task)
	}
}

// GetTask returns a snapshot copy of a task.
func GetTask(taskID string) (*BackgroundTask, bool) {
	backgroundRegistry.mu.Lock()
	defer backgroundRegistry.mu.Unlock()
	t, ok := backgroundRegistry.tasks[taskID]
	return t, ok
}

// WaitTask blocks until the task reaches a terminal state or the timeout
// elapses. timeout <= 0 waits indefinitely (bounded by ctx).
func WaitTask(ctx context.Context, taskID string, timeout time.Duration) (*BackgroundTask, bool) {
	backgroundRegistry.mu.Lock()
	t, ok := backgroundRegistry.tasks[taskID]
	backgroundRegistry.mu.Unlock()
	if !ok {
		return nil, false
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	select {
	case <-t.done:
	case <-ctx.Done():
	}
	return t, true
}

// StopTask kills a running task's whole process group. Returns false when the
// task is unknown or already terminal.
func StopTask(taskID string) bool {
	backgroundRegistry.mu.Lock()
	t, ok := backgroundRegistry.tasks[taskID]
	backgroundRegistry.mu.Unlock()
	if !ok || t.Status != TaskRunning {
		return false
	}
	t.cancel()
	// Negative PID targets the whole process group (Setpgid above).
	if t.cmd != nil && t.cmd.Process != nil {
		_ = syscall.Kill(-t.cmd.Process.Pid, syscall.SIGTERM)
		go func() {
			select {
			case <-t.done:
			case <-time.After(3 * time.Second):
				_ = syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL)
			}
		}()
	}
	return true
}

// ListTasks returns tasks for a session (all sessions when sessionID is
// empty), newest first.
func ListTasks(sessionID string) []*BackgroundTask {
	backgroundRegistry.mu.Lock()
	defer backgroundRegistry.mu.Unlock()
	out := make([]*BackgroundTask, 0, len(backgroundRegistry.tasks))
	for _, t := range backgroundRegistry.tasks {
		if sessionID != "" && t.SessionID != sessionID {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out
}

// TaskNotification is a completed-task summary drained into the model's
// context (ZCode's <task-notification> pattern).
type TaskNotification struct {
	TaskID   string
	Command  string
	Status   TaskStatus
	ExitCode int
	Tail     string
}

// DrainTaskNotifications returns notifications for this session's tasks that
// reached a terminal state since the last drain, marking them consumed. The
// agent calls this at the start of every model step so the model learns a
// background command's outcome without polling.
func DrainTaskNotifications(sessionID string) []TaskNotification {
	if sessionID == "" {
		return nil
	}
	backgroundRegistry.mu.Lock()
	defer backgroundRegistry.mu.Unlock()
	ids := make([]string, 0)
	for id, t := range backgroundRegistry.tasks {
		if t.SessionID == sessionID && t.Status != TaskRunning && !t.notified {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	notes := make([]TaskNotification, 0, len(ids))
	for _, id := range ids {
		t := backgroundRegistry.tasks[id]
		t.notified = true
		notes = append(notes, TaskNotification{
			TaskID:   t.ID,
			Command:  t.Command,
			Status:   t.Status,
			ExitCode: t.ExitCode,
			Tail:     t.OutputTail,
		})
	}
	return notes
}

// taskSnapshot builds the JSON-friendly map returned by the task tools.
func (t *BackgroundTask) taskSnapshot() map[string]any {
	size := t.OutputSize
	if t.Status == TaskRunning && t.OutputFile != "" {
		size = fileSizeOrZero(t.OutputFile)
	}
	out := map[string]any{
		"task_id":     t.ID,
		"command":     t.Command,
		"status":      string(t.Status),
		"started_at":  t.StartedAt.Format(time.RFC3339),
		"output_size": size,
	}
	if t.Cwd != "" {
		out["cwd"] = t.Cwd
	}
	if t.Finished != nil {
		out["finished_at"] = t.Finished.Format(time.RFC3339)
	}
	if t.Status != TaskRunning {
		out["exit_code"] = t.ExitCode
	}
	tail := t.OutputTail
	if tail == "" && t.tail != nil {
		tail = t.tail.tail()
	}
	if tail != "" {
		out["output_tail"] = tail
	}
	if t.OutputFile != "" {
		out["output_file"] = t.OutputFile
	}
	return out
}

// readTailFile returns the last `max` bytes of a file, split on line boundaries.
func readTailFile(path string, max int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := fi.Size()
	offset := size - max
	if offset < 0 {
		offset = 0
	}
	buf := make([]byte, size-offset)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return "", err
	}
	return string(buf), nil
}
