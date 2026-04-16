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
		if triggerList {
			return listSessions()
		}
		if len(args) < 1 {
			return cmd.Help()
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
	rootCmd.AddCommand(triggerCmd)
}

func runTrigger(effectiveID, message string) error {
	CleanStaleSessions()

	info, err := LookupSession(effectiveID)
	if err != nil {
		return err
	}
	_ = info

	userMessage := message
	if userMessage == "" {
		userMessage = "wake"
	}
	if err := WriteTrigger(effectiveID, userMessage); err != nil {
		return err
	}

	if message != "" {
		fmt.Fprintf(os.Stderr, "triggered %q with message\n", effectiveID)
	} else {
		fmt.Fprintf(os.Stderr, "triggered %q\n", effectiveID)
	}
	return nil
}

func listSessions() error {
	CleanStaleSessions()
	dir := sessionsDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	found := false
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
		fmt.Fprintf(os.Stderr, "%-25s  pid %-7d  type %-9s  dir %s\n", eid, info.PID, info.Type, info.WorkDir)
		found = true
	}
	if !found {
		fmt.Fprintln(os.Stderr, "no running sessions")
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
