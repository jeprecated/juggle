package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline <node> [node...]",
	Short: "Run an agent pipeline defined as ordered agent and cmd nodes",
	Long: `Run a pipeline of agent and cmd nodes with lifecycle events, dependencies,
and failure policies.

Each node is declared with either the 'agent' or 'cmd' keyword followed by
a name, a prompt or command, and optional node flags.

Example:

  juggle pipeline \
    agent "Setup" @setup.md --event run-start --model haiku \
    agent "Implement" @task.md --event loop-body \
    cmd "Commit" "git add -A && git commit -m done" --event loop-end

Node kinds:
  agent   Run an AI agent for this step
  cmd     Run a shell command for this step

Shared node flags:
  --event       Lifecycle event (run-start, loop-start, loop-body, loop-end, run-end, failure)
  --after       Explicit dependency on a named node (repeatable)
  --parallel    Suppress implicit previous-node dependency
  --when        Condition expression (e.g. iteration==1)
  --on-failure  Failure policy: stop, continue, retry (default: stop)
  --retries     Retry count when on-failure=retry
  --timeout     Per-node timeout
  --workdir     Working directory for this node`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPipelineCmd(cmd, args)
	},
}

func init() {
	pipelineCmd.SetHelpFunc(groupedHelp)
	rootCmd.AddCommand(pipelineCmd)
}

func runPipelineCmd(_ *cobra.Command, _ []string) error {
	fmt.Fprintln(os.Stderr, "juggle pipeline: execution not yet implemented")
	return nil
}
