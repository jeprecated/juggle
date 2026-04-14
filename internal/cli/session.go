package cli

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type SessionInfo struct {
	PID       int      `json:"pid"`
	Type      string   `json:"type"`
	WatchDirs []string `json:"watch_dirs,omitempty"`
	WorkDir   string   `json:"workdir"`
	Prompt    string   `json:"prompt,omitempty"`
	StartedAt string   `json:"started_at"`
}

func sessionsDir() string {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "juggle", "sessions")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "state", "juggle", "sessions")
}

// EffectiveID computes a session identifier from a user-given name and the
// working directory. The result is "<basedir>-<name>" where basedir is the
// last path component of workdir. Namespaced by basename is sufficient for
// single-user workstations where juggle runs. On multi-user systems, users
// should not share the same XDG_STATE_HOME.
func EffectiveID(name, workdir string) string {
	base := filepath.Base(workdir)
	if base == "" || base == "." || base == "/" {
		base = "root"
	}
	return base + "-" + name
}

func sessionFilePath(effectiveID string) string {
	return filepath.Join(sessionsDir(), effectiveID+".json")
}

func wakeFilePath(effectiveID string) string {
	return filepath.Join(sessionsDir(), effectiveID+".wake")
}

func inboxDir(effectiveID string) string {
	return filepath.Join(sessionsDir(), effectiveID+".d")
}

func lockFilePath(effectiveID string) string {
	return filepath.Join(sessionsDir(), effectiveID+".lock")
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// RegisterSession atomically claims a session slot using an exclusive lock file.
// If a session with the same effectiveID is already running (live PID), it errors.
// If stale (dead PID), the old session and its inbox are cleaned up first.
func RegisterSession(effectiveID string, info SessionInfo) error {
	dir := sessionsDir()
	if dir == "" {
		return fmt.Errorf("cannot determine session directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	// Acquire an exclusive lock to prevent races between concurrent registrations.
	lockFile, err := os.OpenFile(lockFilePath(effectiveID), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			// Lock file exists — check if the holder is alive
			data, readErr := os.ReadFile(lockFilePath(effectiveID))
			if readErr == nil {
				var holder struct {
					PID int `json:"pid"`
				}
				if json.Unmarshal(data, &holder) == nil && isProcessAlive(holder.PID) {
					return fmt.Errorf("session %q already running (pid %d)", effectiveID, holder.PID)
				}
			}
			// Stale lock: remove and retry
			_ = os.Remove(lockFilePath(effectiveID))
			lockFile, err = os.OpenFile(lockFilePath(effectiveID), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("session %q: could not acquire lock: %w", effectiveID, err)
			}
		} else {
			return fmt.Errorf("session %q: lock: %w", effectiveID, err)
		}
	}
	// Write our PID into the lock file for diagnostics
	_, _ = fmt.Fprintf(lockFile, `{"pid":%d}`, info.PID)
	_ = lockFile.Close()

	sp := sessionFilePath(effectiveID)

	// Check for an existing session file (may be stale from a prior crash).
	if data, err := os.ReadFile(sp); err == nil {
		var existing SessionInfo
		if json.Unmarshal(data, &existing) == nil {
			if isProcessAlive(existing.PID) {
				_ = os.Remove(lockFilePath(effectiveID))
				return fmt.Errorf("session %q already running (pid %d)", effectiveID, existing.PID)
			}
			cleanSessionFiles(effectiveID)
		}
	}

	ibDir := inboxDir(effectiveID)
	if err := os.MkdirAll(ibDir, 0700); err != nil {
		_ = os.Remove(lockFilePath(effectiveID))
		return fmt.Errorf("create inbox dir: %w", err)
	}

	data, err := json.Marshal(info)
	if err != nil {
		_ = os.Remove(lockFilePath(effectiveID))
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := os.WriteFile(sp, data, 0600); err != nil {
		_ = os.Remove(lockFilePath(effectiveID))
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

// UnregisterSession removes the session JSON, wake file, lock file, and all inbox contents.
func UnregisterSession(effectiveID string) {
	_ = os.Remove(sessionFilePath(effectiveID))
	_ = os.Remove(wakeFilePath(effectiveID))
	_ = os.Remove(lockFilePath(effectiveID))
	ibDir := inboxDir(effectiveID)
	entries, err := os.ReadDir(ibDir)
	if err == nil {
		for _, e := range entries {
			_ = os.Remove(filepath.Join(ibDir, e.Name()))
		}
	}
	_ = os.Remove(ibDir)
}

func cleanSessionFiles(effectiveID string) {
	_ = os.Remove(sessionFilePath(effectiveID))
	_ = os.Remove(wakeFilePath(effectiveID))
	ibDir := inboxDir(effectiveID)
	entries, err := os.ReadDir(ibDir)
	if err == nil {
		for _, e := range entries {
			_ = os.Remove(filepath.Join(ibDir, e.Name()))
		}
	}
	_ = os.Remove(ibDir)
}

func LookupSession(effectiveID string) (SessionInfo, error) {
	sp := sessionFilePath(effectiveID)
	data, err := os.ReadFile(sp)
	if err != nil {
		return SessionInfo{}, fmt.Errorf("session %q not found", effectiveID)
	}
	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return SessionInfo{}, fmt.Errorf("session %q: invalid data", effectiveID)
	}
	if !isProcessAlive(info.PID) {
		cleanSessionFiles(effectiveID)
		return SessionInfo{}, fmt.Errorf("session %q is stale (pid %d dead), cleaned up", effectiveID, info.PID)
	}
	return info, nil
}

func CheckWake(effectiveID string) bool {
	wp := wakeFilePath(effectiveID)
	if _, err := os.Stat(wp); err != nil {
		return false
	}
	_ = os.Remove(wp)
	return true
}

// ReadTrigger reads and consumes the oldest trigger message from the inbox.
// It uses an atomic rename to claim the file, preventing concurrent workers
// from reading the same message. Returns ("", nil) when the inbox is empty.
func ReadTrigger(effectiveID string) (string, error) {
	ibDir := inboxDir(effectiveID)
	entries, err := os.ReadDir(ibDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read inbox: %w", err)
	}
	if len(entries) == 0 {
		return "", nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		oldest := filepath.Join(ibDir, entry.Name())
		claimed := oldest + ".claimed"
		if err := os.Rename(oldest, claimed); err != nil {
			continue
		}
		data, err := os.ReadFile(claimed)
		_ = os.Remove(claimed)
		if err != nil {
			return "", fmt.Errorf("read trigger: %w", err)
		}
		return string(data), nil
	}

	return "", nil
}

func WriteTrigger(effectiveID, message string) error {
	ibDir := inboxDir(effectiveID)
	if err := os.MkdirAll(ibDir, 0700); err != nil {
		return fmt.Errorf("create inbox: %w", err)
	}

	var b [4]byte
	_, _ = rand.Read(b[:])
	suffix := fmt.Sprintf("%04x", b)
	ts := time.Now()
	filename := fmt.Sprintf("%s-%s-%s.md",
		ts.Format("20060102"),
		ts.Format("150405"),
		suffix,
	)

	path := filepath.Join(ibDir, filename)
	if err := os.WriteFile(path, []byte(message), 0600); err != nil {
		return fmt.Errorf("write trigger: %w", err)
	}

	wp := wakeFilePath(effectiveID)
	f, err := os.OpenFile(wp, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("touch wake: %w", err)
	}
	_ = f.Close()
	return nil
}

func FormatTrigger(message string) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	escaped := escapeXML(strings.TrimSpace(message))
	return fmt.Sprintf("<trigger sent-at=\"%s\">\n%s\n</trigger>", ts, escaped)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func CleanStaleSessions() {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var info SessionInfo
		if json.Unmarshal(data, &info) != nil {
			continue
		}
		if !isProcessAlive(info.PID) {
			eid := strings.TrimSuffix(e.Name(), ".json")
			cleanSessionFiles(eid)
			_ = os.Remove(lockFilePath(eid))
		}
	}
}
