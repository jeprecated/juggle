package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
)

// isGlobPattern reports whether s contains any glob metacharacter (* ? [).
func isGlobPattern(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// FindVCSRoot walks up from dir looking for a .git or .jj marker directory.
// Returns the directory containing the marker, or "" if none found before the filesystem root.
func FindVCSRoot(dir string) string {
	dir = filepath.Clean(dir)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".jj")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// expandGlobDirs expands pattern relative to basedir and returns only directories matching the pattern.
// If basedir is empty, the current working directory is used.
// Supports ** via the doublestar library.
func expandGlobDirs(basedir, pattern string) ([]string, error) {
	if basedir == "" {
		var err error
		basedir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}

	fsys := os.DirFS(basedir)
	matches, err := doublestar.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %q: %w", pattern, err)
	}

	var dirs []string
	for _, m := range matches {
		abs := filepath.Join(basedir, m)
		info, err := os.Stat(abs)
		if err == nil && info.IsDir() {
			dirs = append(dirs, abs)
		}
	}
	return dirs, nil
}

// claimFromDirs scans each dir in dirs and atomically claims the first unclaimed file.
// Inaccessible directories are silently skipped. Returns "" if nothing is available.
func (c *workerCoordinator) claimFromDirs(dirs []string) (string, error) {
	var allFiles []string
	for _, dir := range dirs {
		files, err := ScanWatchDirAll(dir)
		if err != nil {
			continue // skip inaccessible dirs
		}
		allFiles = append(allFiles, files...)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for _, f := range allFiles {
		if !c.claimed[f] {
			c.claimed[f] = true
			return f, nil
		}
	}
	return "", nil
}

// runGlobWatch handles --watch with a glob pattern. It periodically expands the
// glob to discover directories (including new ones), picks task files, sets the
// agent's working directory to the VCS root of the matched task, and respects
// --workers for concurrency.
func runGlobWatch(cfg Config) error {
	if cfg.Workers > 1 {
		return runGlobWatchWorkers(cfg)
	}
	// Auto-enable dashboard for glob watch (output would otherwise be unreadable
	// as tasks from different dirs interleave with the same stderr prefix).
	cfg.Dashboard = true
	return runGlobWatchSerial(cfg)
}

func runGlobWatchSerial(cfg Config) error {
	globPattern := cfg.Watch[0]

	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	var dash *workerDashboard
	if cfg.Dashboard {
		dash = setupWorkerDashboard(globPattern, 1, cfg.Stderr)
		defer dash.stop()
		if logFile, closeLog := dash.openWorkerLog(0); logFile != nil {
			cfg.Stderr = logFile
			defer closeLog()
		}
	}

	stats := runStats{runID: cfg.RunID, start: time.Now(), model: cfg.Model}

	for {
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		dirs, err := expandGlobDirs(cfg.WorkDir, globPattern)
		if err != nil {
			return fmt.Errorf("expanding glob %q: %w", globPattern, err)
		}

		taskPath := ""
		for _, dir := range dirs {
			p, err := ScanWatchDir(dir)
			if err != nil {
				continue
			}
			if p != "" {
				taskPath = p
				break
			}
		}

		if taskPath == "" {
			if dash != nil {
				dash.dash.Update(0, WorkerState{Status: WorkerIdle, LogFile: dash.logFiles[0]})
			}
			fmt.Fprintf(cfg.Stderr, "No tasks found matching %s, polling in %v...\n", globPattern, pollDelay)
			select {
			case <-time.After(pollDelay):
			case <-cfg.Shutdown:
			}
			continue
		}

		taskCfg := cfg
		if vcsRoot := FindVCSRoot(filepath.Dir(taskPath)); vcsRoot != "" {
			taskCfg.WorkDir = vcsRoot
		}

		filename := filepath.Base(taskPath)
		if dash != nil {
			logFile := dash.logFiles[0]
			dash.dash.Update(0, WorkerState{
				Status:    WorkerActive,
				TaskName:  filename,
				Iteration: 0,
				MaxIter:   cfg.Iterations,
				LogFile:   logFile,
			})
			taskCfg.OnIterDone = func(iter, maxIter int) {
				dash.dash.Update(0, WorkerState{
					Status:    WorkerActive,
					TaskName:  filename,
					Iteration: iter,
					MaxIter:   maxIter,
					LogFile:   logFile,
				})
			}
		}
		fmt.Fprintf(cfg.Stderr, "Processing task: %s\n", filename)

		if err := runWatchTask(taskCfg, taskPath, filename, &stats); err != nil {
			if dash != nil {
				dash.dash.Update(0, WorkerState{Status: WorkerIdle, LogFile: dash.logFiles[0]})
			}
			if errors.Is(err, ErrInterrupted) {
				writeSummary(cfg, stats)
				return ErrInterrupted
			}
			if errors.Is(err, errCostGuard) {
				writeSummary(cfg, stats)
				return nil
			}
			fmt.Fprintf(cfg.Stderr, "Error processing %s: %v\n", filename, err)
			continue
		}
		if dash != nil {
			dash.dash.Update(0, WorkerState{Status: WorkerIdle, LogFile: dash.logFiles[0]})
		}
	}
}

func runGlobWatchWorkers(cfg Config) error {
	globPattern := cfg.Watch[0]

	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	// Dashboard is auto-enabled for glob watch workers (multiple workers = interleaved output).
	cfg.Dashboard = true
	dash := setupWorkerDashboard(globPattern, cfg.Workers, cfg.Stderr)
	defer dash.stop()

	coord := newWorkerCoordinator()
	errs := make(chan error, cfg.Workers)
	var wg sync.WaitGroup

	getDirs := func() []string {
		dirs, _ := expandGlobDirs(cfg.WorkDir, globPattern)
		return dirs
	}

	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerCfg := cfg
			logFile, closeLog := dash.openWorkerLog(workerID)
			defer closeLog()
			if logFile != nil {
				workerCfg.Stderr = logFile
			}
			workerCfg.Runner = &workerIDRunner{inner: cfg.Runner, workerID: workerID}
			if err := runGlobWorkerLoop(workerCfg, getDirs, coord, pollDelay, dash, workerID); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}
	return nil
}

func runGlobWorkerLoop(cfg Config, getDirs func() []string, coord *workerCoordinator, pollDelay time.Duration, dash *workerDashboard, workerID int) error {
	stats := runStats{runID: cfg.RunID, start: time.Now(), model: cfg.Model}

	logFile := ""
	if dash != nil && workerID >= 0 && workerID < len(dash.logFiles) {
		logFile = dash.logFiles[workerID]
	}

	for {
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		dirs := getDirs()
		taskPath, err := coord.claimFromDirs(dirs)
		if err != nil {
			return err
		}

		if taskPath == "" {
			if dash != nil {
				dash.dash.Update(workerID, WorkerState{Status: WorkerIdle, LogFile: logFile})
			}
			fmt.Fprintf(cfg.Stderr, "No tasks available, polling in %v...\n", pollDelay)
			select {
			case <-time.After(pollDelay):
			case <-cfg.Shutdown:
			}
			continue
		}

		taskCfg := cfg
		if vcsRoot := FindVCSRoot(filepath.Dir(taskPath)); vcsRoot != "" {
			taskCfg.WorkDir = vcsRoot
		}

		filename := filepath.Base(taskPath)
		if dash != nil {
			dash.dash.Update(workerID, WorkerState{
				Status:    WorkerActive,
				TaskName:  filename,
				Iteration: 0,
				MaxIter:   cfg.Iterations,
				LogFile:   logFile,
			})
			taskCfg.OnIterDone = func(iter, maxIter int) {
				dash.dash.Update(workerID, WorkerState{
					Status:    WorkerActive,
					TaskName:  filename,
					Iteration: iter,
					MaxIter:   maxIter,
					LogFile:   logFile,
				})
			}
		}
		fmt.Fprintf(cfg.Stderr, "Processing task: %s\n", filename)

		if err := runWatchTask(taskCfg, taskPath, filename, &stats); err != nil {
			coord.release(taskPath)
			if dash != nil {
				dash.dash.Update(workerID, WorkerState{Status: WorkerIdle, LogFile: logFile})
			}
			if errors.Is(err, ErrInterrupted) {
				writeSummary(cfg, stats)
				return ErrInterrupted
			}
			if errors.Is(err, errCostGuard) {
				writeSummary(cfg, stats)
				return nil
			}
			fmt.Fprintf(cfg.Stderr, "Error processing %s: %v\n", filename, err)
			continue
		}
		coord.release(taskPath)
		if dash != nil {
			dash.dash.Update(workerID, WorkerState{Status: WorkerIdle, LogFile: logFile})
		}
	}
}

// runMultiWatch handles --watch specified more than once. It merges all watch
// entries (expanding any glob patterns each poll cycle) into a shared directory
// list, and uses claimFromDirs so a shared worker pool picks across all dirs.
// Dashboard is auto-enabled to handle interleaved output from multiple dirs.
func runMultiWatch(cfg Config) error {
	// Auto-enable dashboard for multi-watch (same as glob watch).
	cfg.Dashboard = true

	if cfg.Workers > 1 {
		return runMultiWatchWorkers(cfg)
	}
	return runMultiWatchSerial(cfg)
}

// getDirsForWatch expands each watch entry: glob patterns are expanded against
// workdir, literal paths are included as-is. Returns the merged directory list.
func getDirsForWatch(watches []string, workDir string) []string {
	var dirs []string
	for _, w := range watches {
		if isGlobPattern(w) {
			expanded, _ := expandGlobDirs(workDir, w)
			dirs = append(dirs, expanded...)
		} else {
			dirs = append(dirs, w)
		}
	}
	return dirs
}

func runMultiWatchSerial(cfg Config) error {
	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	title := strings.Join(cfg.Watch, ", ")

	var dash *workerDashboard
	if cfg.Dashboard {
		dash = setupWorkerDashboard(title, 1, cfg.Stderr)
		defer dash.stop()
		if logFile, closeLog := dash.openWorkerLog(0); logFile != nil {
			cfg.Stderr = logFile
			defer closeLog()
		}
	}

	stats := runStats{runID: cfg.RunID, start: time.Now(), model: cfg.Model}
	coord := newWorkerCoordinator()

	for {
		select {
		case <-cfg.Shutdown:
			writeSummary(cfg, stats)
			return ErrInterrupted
		default:
		}

		dirs := getDirsForWatch(cfg.Watch, cfg.WorkDir)
		taskPath, err := coord.claimFromDirs(dirs)
		if err != nil {
			return err
		}

		if taskPath == "" {
			if dash != nil {
				dash.dash.Update(0, WorkerState{Status: WorkerIdle, LogFile: dash.logFiles[0]})
			}
			fmt.Fprintf(cfg.Stderr, "No tasks found in watched dirs, polling in %v...\n", pollDelay)
			select {
			case <-time.After(pollDelay):
			case <-cfg.Shutdown:
			}
			continue
		}

		taskCfg := cfg
		if vcsRoot := FindVCSRoot(filepath.Dir(taskPath)); vcsRoot != "" {
			taskCfg.WorkDir = vcsRoot
		}

		filename := filepath.Base(taskPath)
		if dash != nil {
			logFile := dash.logFiles[0]
			dash.dash.Update(0, WorkerState{
				Status:    WorkerActive,
				TaskName:  filename,
				Iteration: 0,
				MaxIter:   cfg.Iterations,
				LogFile:   logFile,
			})
			taskCfg.OnIterDone = func(iter, maxIter int) {
				dash.dash.Update(0, WorkerState{
					Status:    WorkerActive,
					TaskName:  filename,
					Iteration: iter,
					MaxIter:   maxIter,
					LogFile:   logFile,
				})
			}
		}
		fmt.Fprintf(cfg.Stderr, "Processing task: %s\n", filename)

		if err := runWatchTask(taskCfg, taskPath, filename, &stats); err != nil {
			coord.release(taskPath)
			if dash != nil {
				dash.dash.Update(0, WorkerState{Status: WorkerIdle, LogFile: dash.logFiles[0]})
			}
			if errors.Is(err, ErrInterrupted) {
				writeSummary(cfg, stats)
				return ErrInterrupted
			}
			if errors.Is(err, errCostGuard) {
				writeSummary(cfg, stats)
				return nil
			}
			fmt.Fprintf(cfg.Stderr, "Error processing %s: %v\n", filename, err)
			continue
		}
		coord.release(taskPath)
		if dash != nil {
			dash.dash.Update(0, WorkerState{Status: WorkerIdle, LogFile: dash.logFiles[0]})
		}
	}
}

func runMultiWatchWorkers(cfg Config) error {
	pollDelay := time.Duration(cfg.Delay) * time.Minute
	if pollDelay < 30*time.Second {
		pollDelay = 30 * time.Second
	}

	title := strings.Join(cfg.Watch, ", ")
	dash := setupWorkerDashboard(title, cfg.Workers, cfg.Stderr)
	defer dash.stop()

	coord := newWorkerCoordinator()
	errs := make(chan error, cfg.Workers)
	var wg sync.WaitGroup

	getDirs := func() []string {
		return getDirsForWatch(cfg.Watch, cfg.WorkDir)
	}

	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerCfg := cfg
			logFile, closeLog := dash.openWorkerLog(workerID)
			defer closeLog()
			if logFile != nil {
				workerCfg.Stderr = logFile
			}
			workerCfg.Runner = &workerIDRunner{inner: cfg.Runner, workerID: workerID}
			if err := runGlobWorkerLoop(workerCfg, getDirs, coord, pollDelay, dash, workerID); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}
	return nil
}

