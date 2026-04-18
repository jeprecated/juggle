package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var triggerCmd = &cobra.Command{
	Use:   "trigger <session-id> [message]",
	Short: "Wake a running juggle session, optionally delivering a message",
	Long: `Trigger a running juggle session identified by its effective ID.

Without a message, wakes the session (skips any delay between iterations).
With a message, queues it for the next iteration and wakes the session.

Use --list to show running sessions.`,
	Example: `  # Wake a session immediately
  juggle trigger cli-dev

  # Send a message to a session
  juggle trigger cli-dev "Fix the order parser crash"

  # List running sessions
  juggle trigger --list`,
	Args:              cobra.RangeArgs(0, 2),
	SilenceUsage:      true,
	SilenceErrors:     true,
	ValidArgsFunction: completeTriggerIDs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if triggerList || len(args) < 1 {
			return listSessions()
		}
		effectiveID := args[0]
		message := ""
		if len(args) > 1 {
			message = args[1]
		}
		return runTrigger(effectiveID, message)
	},
}

var triggerList bool

func init() {
	triggerCmd.Flags().BoolVar(&triggerList, "list", false, "list running sessions")
	triggerCmd.SetHelpFunc(groupedHelp)
	// Exclude all inherited persistent flags that are irrelevant to trigger.
	// Only --format and --list should appear in help.
	triggerCmd.Annotations = map[string]string{
		excludeFlagsKey: strings.Join([]string{
			"model", "provider", "trust", "plan", "interactive", "timeout", "max-wait",
			"dry-run", "show-thinking", "verbose", "max-failures", "cmd-before", "cmd-after",
			"stop-when", "agent-pre", "agent-before", "agent-after", "agent-post", "hook",
			"hooks-file", "log", "max-cost", "label", "allowed-tools", "disallowed-tools",
			"max-turns", "mcp-config", "on-failure", "retries", "agent-cmd", "command",
			"system-prompt", "retry-prompt", "workdir", "channels", "extra", "no-config",
			"no-log", "id",
		}, ","),
	}
	rootCmd.AddCommand(triggerCmd)
}

// sessionEntry holds a session's effective ID alongside its info.
type sessionEntry struct {
	ID          string
	PID         int
	Type        string
	WorkDir     string
	WatchDirs   []string `json:",omitempty"`
	StartedAt   string
}

func runTrigger(effectiveID, message string) error {
	CleanStaleSessions()

	info, err := LookupSession(effectiveID)
	if err != nil {
		if agentFormat() == FormatToon {
			code := "not_found"
			if strings.Contains(err.Error(), "stale") {
				code = "stale"
			}
			ToonError(os.Stdout, code, err.Error())
			return err
		}
		return err
	}
	_ = info

	userMessage := message
	if userMessage == "" {
		userMessage = "wake"
	}
	if err := WriteTrigger(effectiveID, userMessage); err != nil {
		if agentFormat() == FormatToon {
			ToonError(os.Stdout, "internal", err.Error())
		}
		return err
	}

	switch agentFormat() {
	case FormatToon:
		status := "triggered"
		if message != "" {
			ToonObject(os.Stdout, []string{"id", "message", "status"}, []string{effectiveID, message, status})
		} else {
			ToonObject(os.Stdout, []string{"id", "status"}, []string{effectiveID, status})
		}
	case FormatJSON:
		obj := map[string]string{"id": effectiveID, "status": "triggered"}
		if message != "" {
			obj["message"] = message
		}
		if err := json.NewEncoder(os.Stdout).Encode(obj); err != nil {
				return err
			}
	default:
		if message != "" {
			fmt.Fprintf(os.Stderr, "triggered %q with message\n", effectiveID)
		} else {
			fmt.Fprintf(os.Stderr, "triggered %q\n", effectiveID)
		}
	}
	return nil
}

// gatherSessions reads all active sessions from the session directory.
func gatherSessions() []sessionEntry {
	CleanStaleSessions()
	dir := sessionsDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []sessionEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := dir + "/" + e.Name()
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var info SessionInfo
		if json.Unmarshal(data, &info) != nil {
			continue
		}
		if !isProcessAlive(info.PID) {
			continue
		}
		eid := strings.TrimSuffix(e.Name(), ".json")
		result = append(result, sessionEntry{
			ID:        eid,
			PID:       info.PID,
			Type:      info.Type,
			WorkDir:   info.WorkDir,
			WatchDirs: info.WatchDirs,
			StartedAt: info.StartedAt,
		})
	}
	return result
}

func listSessions() error {
	sessions := gatherSessions()

	switch agentFormat() {
	case FormatToon:
		if len(sessions) == 0 {
			fmt.Fprintln(os.Stdout, "sessions[0]: none found")
			return nil
		}
		fields := []string{"id", "type", "pid", "workdir"}
		rows := make([][]string, len(sessions))
		for i, s := range sessions {
			rows[i] = []string{s.ID, s.Type, fmt.Sprintf("%d", s.PID), s.WorkDir}
		}
		ToonList(os.Stdout, "sessions", fields, rows, 0)
		ToonHelp(os.Stdout, []string{
			"juggle trigger <id> \"message\" to send a trigger",
		})

	case FormatJSON:
		if len(sessions) == 0 {
			fmt.Fprintln(os.Stdout, "[]")
			return nil
		}
		if err := json.NewEncoder(os.Stdout).Encode(sessions); err != nil {
				return err
			}

	default:
		if len(sessions) == 0 {
			fmt.Fprintln(os.Stderr, "no running sessions")
			return nil
		}
		for _, s := range sessions {
			fmt.Fprintf(os.Stderr, "%-25s  pid %-7d  type %-9s  dir %s\n", s.ID, s.PID, s.Type, s.WorkDir)
		}
	}
	return nil
}

func completeTriggerIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	dir := sessionsDir()
	if dir == "" {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		eid := strings.TrimSuffix(e.Name(), ".json")
		if strings.HasPrefix(eid, toComplete) {
			ids = append(ids, eid)
		}
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}
