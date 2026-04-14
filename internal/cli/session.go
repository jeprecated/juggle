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

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func RegisterSession(effectiveID string, info SessionInfo) error {
	dir := sessionsDir()
	if dir == "" {
		return fmt.Errorf("cannot determine session directory")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	sp := sessionFilePath(effectiveID)

	if data, err := os.ReadFile(sp); err == nil {
		var existing SessionInfo
		if json.Unmarshal(data, &existing) == nil {
			if isProcessAlive(existing.PID) {
				return fmt.Errorf("session %q already running (pid %d)", effectiveID, existing.PID)
			}
			cleanSessionFiles(effectiveID)
		}
	}

	ibDir := inboxDir(effectiveID)
	if err := os.MkdirAll(ibDir, 0755); err != nil {
		return fmt.Errorf("create inbox dir: %w", err)
	}

	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return os.WriteFile(sp, data, 0644)
}

func UnregisterSession(effectiveID string) {
	_ = os.Remove(sessionFilePath(effectiveID))
	_ = os.Remove(wakeFilePath(effectiveID))
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

	oldest := filepath.Join(ibDir, entries[0].Name())
	data, err := os.ReadFile(oldest)
	if err != nil {
		return "", fmt.Errorf("read trigger: %w", err)
	}
	_ = os.Remove(oldest)
	return string(data), nil
}

func WriteTrigger(effectiveID, message string) error {
	ibDir := inboxDir(effectiveID)
	if err := os.MkdirAll(ibDir, 0755); err != nil {
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
	if err := os.WriteFile(path, []byte(message), 0644); err != nil {
		return fmt.Errorf("write trigger: %w", err)
	}

	wp := wakeFilePath(effectiveID)
	f, err := os.Create(wp)
	if err != nil {
		return fmt.Errorf("touch wake: %w", err)
	}
	_ = f.Close()
	return nil
}

func FormatTrigger(message string) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	return fmt.Sprintf("<trigger sent-at=\"%s\">\n%s\n</trigger>", ts, strings.TrimSpace(message))
}

func CleanStaleSessions() {
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".json") {
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
		}
	}
}
