package cli

import (
	"strings"
	"testing"
)

func TestNewDashboard_hasIdleWorkers(t *testing.T) {
	d := NewDashboard("/watch/dir", 3)
	for i := 0; i < 3; i++ {
		state := d.WorkerState(i)
		if state.Status != WorkerIdle {
			t.Errorf("worker %d: expected idle, got %s", i, state.Status)
		}
	}
}

func TestDashboard_renderShowsWatchDir(t *testing.T) {
	d := NewDashboard("/my/watch/dir", 1)
	out := d.Render()
	if !strings.Contains(out, "/my/watch/dir") {
		t.Errorf("expected watch dir in render output, got:\n%s", out)
	}
}

func TestDashboard_renderShowsWorkerCount(t *testing.T) {
	d := NewDashboard("/watch/dir", 4)
	out := d.Render()
	if !strings.Contains(out, "4") {
		t.Errorf("expected worker count (4) in render output, got:\n%s", out)
	}
}

func TestDashboard_renderShowsIdleWorkers(t *testing.T) {
	d := NewDashboard("/watch/dir", 2)
	out := d.Render()
	if !strings.Contains(out, "idle") {
		t.Errorf("expected 'idle' status in render output, got:\n%s", out)
	}
}

func TestDashboard_updateMakesWorkerActive(t *testing.T) {
	d := NewDashboard("/watch/dir", 2)
	d.Update(0, WorkerState{Status: WorkerActive, TaskName: "my-task.md", Iteration: 1, MaxIter: 5})
	state := d.WorkerState(0)
	if state.Status != WorkerActive {
		t.Errorf("expected active, got %s", state.Status)
	}
	if state.TaskName != "my-task.md" {
		t.Errorf("expected task name 'my-task.md', got %q", state.TaskName)
	}
}

func TestDashboard_renderShowsActiveTask(t *testing.T) {
	d := NewDashboard("/watch/dir", 2)
	d.Update(0, WorkerState{Status: WorkerActive, TaskName: "big-task.md", Iteration: 3, MaxIter: 10})
	out := d.Render()
	if !strings.Contains(out, "big-task.md") {
		t.Errorf("expected task name in render output, got:\n%s", out)
	}
	if !strings.Contains(out, "active") {
		t.Errorf("expected 'active' status in render output, got:\n%s", out)
	}
}

func TestDashboard_renderShowsIterationProgress(t *testing.T) {
	d := NewDashboard("/watch/dir", 1)
	d.Update(0, WorkerState{Status: WorkerActive, TaskName: "task.md", Iteration: 3, MaxIter: 10})
	out := d.Render()
	if !strings.Contains(out, "3/10") {
		t.Errorf("expected '3/10' iteration progress in render output, got:\n%s", out)
	}
}

func TestDashboard_renderShowsUnlimitedIterations(t *testing.T) {
	d := NewDashboard("/watch/dir", 1)
	d.Update(0, WorkerState{Status: WorkerActive, TaskName: "task.md", Iteration: 5, MaxIter: 0})
	out := d.Render()
	if !strings.Contains(out, "5") {
		t.Errorf("expected iteration number in render output, got:\n%s", out)
	}
}

func TestDashboard_workerGoesIdleAfterTask(t *testing.T) {
	d := NewDashboard("/watch/dir", 1)
	d.Update(0, WorkerState{Status: WorkerActive, TaskName: "task.md", Iteration: 1, MaxIter: 1})
	d.Update(0, WorkerState{Status: WorkerIdle})
	state := d.WorkerState(0)
	if state.Status != WorkerIdle {
		t.Errorf("expected idle after task, got %s", state.Status)
	}
}

func TestDashboard_multipleWorkersIndependent(t *testing.T) {
	d := NewDashboard("/watch/dir", 3)
	d.Update(0, WorkerState{Status: WorkerActive, TaskName: "task-a.md", Iteration: 1, MaxIter: 5})
	d.Update(2, WorkerState{Status: WorkerActive, TaskName: "task-c.md", Iteration: 2, MaxIter: 5})

	if d.WorkerState(0).TaskName != "task-a.md" {
		t.Errorf("worker 0: expected task-a.md, got %q", d.WorkerState(0).TaskName)
	}
	if d.WorkerState(1).Status != WorkerIdle {
		t.Errorf("worker 1: expected idle, got %s", d.WorkerState(1).Status)
	}
	if d.WorkerState(2).TaskName != "task-c.md" {
		t.Errorf("worker 2: expected task-c.md, got %q", d.WorkerState(2).TaskName)
	}
}

func TestDashboard_renderShowsLogFilePaths(t *testing.T) {
	d := NewDashboard("/watch/dir", 2)
	d.Update(0, WorkerState{Status: WorkerActive, TaskName: "task.md", Iteration: 1, MaxIter: 5, LogFile: "/tmp/worker-0.log"})
	out := d.Render()
	if !strings.Contains(out, "/tmp/worker-0.log") {
		t.Errorf("expected log file path in render output, got:\n%s", out)
	}
}

func TestDashboard_renderAllWorkersTable(t *testing.T) {
	d := NewDashboard("/tasks", 2)
	d.Update(0, WorkerState{Status: WorkerActive, TaskName: "task-1.md", Iteration: 2, MaxIter: 5})
	d.Update(1, WorkerState{Status: WorkerIdle})
	out := d.Render()
	// Both workers must appear
	if !strings.Contains(out, "task-1.md") {
		t.Errorf("expected task-1.md in output, got:\n%s", out)
	}
	if !strings.Contains(out, "idle") {
		t.Errorf("expected idle worker in output, got:\n%s", out)
	}
}
