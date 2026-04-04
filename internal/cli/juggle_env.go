package cli

import (
	"crypto/rand"
	"fmt"
)

// generateRunID returns a random UUID v4 string for stable run identification.
func generateRunID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// buildJuggleEnv constructs JUGGLE_* environment variables for a run iteration.
//   - workerID < 0: omit JUGGLE_WORKER_ID (not a worker-pool run)
//   - taskFile == "": omit JUGGLE_TASK_FILE (not a watch-mode run)
//   - label == "": omit JUGGLE_LABEL
func buildJuggleEnv(runID string, iteration, maxIterations int, label, model, providerName, taskFile string, workerID int) []string {
	env := []string{
		fmt.Sprintf("JUGGLE_ITERATION=%d", iteration),
		fmt.Sprintf("JUGGLE_MAX_ITERATIONS=%d", maxIterations),
		fmt.Sprintf("JUGGLE_RUN_ID=%s", runID),
		fmt.Sprintf("JUGGLE_MODEL=%s", model),
		fmt.Sprintf("JUGGLE_PROVIDER=%s", providerName),
	}
	if label != "" {
		env = append(env, fmt.Sprintf("JUGGLE_LABEL=%s", label))
	}
	if taskFile != "" {
		env = append(env, fmt.Sprintf("JUGGLE_TASK_FILE=%s", taskFile))
	}
	if workerID >= 0 {
		env = append(env, fmt.Sprintf("JUGGLE_WORKER_ID=%d", workerID))
	}
	return env
}
