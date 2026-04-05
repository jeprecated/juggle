package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// WorkerStatus represents the current status of a watch worker.
type WorkerStatus string

const (
	// WorkerIdle means the worker is waiting for a task.
	WorkerIdle WorkerStatus = "idle"
	// WorkerActive means the worker is currently processing a task.
	WorkerActive WorkerStatus = "active"
)

// WorkerState holds the current state of a single watch worker.
type WorkerState struct {
	Status    WorkerStatus
	TaskName  string
	Iteration int
	MaxIter   int
	LogFile   string // path to worker output log for tail/drill-down
}

// Dashboard manages a live TUI overview of running watch workers.
type Dashboard struct {
	mu       sync.Mutex
	workers  []WorkerState
	dir      string
	start    time.Time
	updated  chan struct{}
}

// NewDashboard creates a Dashboard tracking numWorkers workers for watchDir.
func NewDashboard(watchDir string, numWorkers int) *Dashboard {
	workers := make([]WorkerState, numWorkers)
	for i := range workers {
		workers[i] = WorkerState{Status: WorkerIdle}
	}
	return &Dashboard{
		workers: workers,
		dir:     watchDir,
		start:   time.Now(),
		updated: make(chan struct{}, 1),
	}
}

// Update sets the state for workerID. Safe to call from multiple goroutines.
func (d *Dashboard) Update(workerID int, state WorkerState) {
	d.mu.Lock()
	if workerID >= 0 && workerID < len(d.workers) {
		d.workers[workerID] = state
	}
	d.mu.Unlock()
	// Non-blocking signal: if channel is full, a render is already pending.
	select {
	case d.updated <- struct{}{}:
	default:
	}
}

// WorkerState returns the current state of workerID. Safe to call from multiple goroutines.
func (d *Dashboard) WorkerState(workerID int) WorkerState {
	d.mu.Lock()
	defer d.mu.Unlock()
	if workerID < 0 || workerID >= len(d.workers) {
		return WorkerState{Status: WorkerIdle}
	}
	return d.workers[workerID]
}

// Render returns a string representation of the dashboard suitable for display.
func (d *Dashboard) Render() string {
	d.mu.Lock()
	workers := make([]WorkerState, len(d.workers))
	copy(workers, d.workers)
	dir := d.dir
	d.mu.Unlock()

	var b strings.Builder
	elapsed := time.Since(d.start)
	fmt.Fprintf(&b, "Watch Dashboard — %s — %d workers — running %s\n\n",
		dir, len(workers), formatElapsed(elapsed))

	fmt.Fprintf(&b, "  %-6s  %-8s  %-24s  %-8s  %s\n",
		"WORKER", "STATUS", "TASK", "ITER", "LOG")

	for i, w := range workers {
		task := w.TaskName
		if task == "" {
			task = "—"
		}
		iter := "—"
		if w.Status == WorkerActive {
			if w.MaxIter > 0 {
				iter = fmt.Sprintf("%d/%d", w.Iteration, w.MaxIter)
			} else {
				iter = fmt.Sprintf("%d", w.Iteration)
			}
		}
		logPath := w.LogFile
		if logPath == "" {
			logPath = "—"
		}
		fmt.Fprintf(&b, "  %-6d  %-8s  %-24s  %-8s  %s\n",
			i, string(w.Status), task, iter, logPath)
	}

	return b.String()
}

// Run renders the dashboard to w in a loop until ctx is cancelled.
// It updates on state changes and at most every 200ms.
func (d *Dashboard) Run(ctx context.Context, w io.Writer) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	render := func() {
		// Clear screen and move cursor to top-left
		fmt.Fprint(w, "\033[2J\033[H")
		fmt.Fprint(w, d.Render())
	}

	render()
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.updated:
			render()
		case <-ticker.C:
			render()
		}
	}
}

// workerDashboard holds runtime state for a dashboard-enabled worker set.
type workerDashboard struct {
	dash     *Dashboard
	logFiles []string  // per-worker log file paths
	cancel   context.CancelFunc
}

// setupWorkerDashboard creates a Dashboard for numWorkers, opens per-worker log files,
// and starts the render goroutine writing to w. Call cleanup() to stop the render loop.
// Worker output should be redirected to the returned log files.
func setupWorkerDashboard(watchDir string, numWorkers int, w io.Writer) *workerDashboard {
	dash := NewDashboard(watchDir, numWorkers)
	logFiles := make([]string, numWorkers)
	for i := 0; i < numWorkers; i++ {
		f, err := os.CreateTemp("", fmt.Sprintf("juggle-worker-%d-*.log", i))
		if err == nil {
			logFiles[i] = f.Name()
			f.Close()
		}
	}
	// Pre-populate dashboard with log file paths so they're visible from the start.
	for i, lf := range logFiles {
		dash.Update(i, WorkerState{Status: WorkerIdle, LogFile: lf})
	}
	ctx, cancel := context.WithCancel(context.Background())
	go dash.Run(ctx, w)
	return &workerDashboard{dash: dash, logFiles: logFiles, cancel: cancel}
}

// stop terminates the dashboard render loop.
func (d *workerDashboard) stop() { d.cancel() }

// openWorkerLog opens the log file for workerID for writing (append).
// Returns nil and a no-op close func on error.
func (d *workerDashboard) openWorkerLog(workerID int) (*os.File, func()) {
	if workerID < 0 || workerID >= len(d.logFiles) || d.logFiles[workerID] == "" {
		return nil, func() {}
	}
	f, err := os.OpenFile(d.logFiles[workerID], os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, func() {}
	}
	return f, func() { f.Close() }
}

// formatElapsed returns a human-readable elapsed time string.
func formatElapsed(d time.Duration) string {
	d = d.Truncate(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
