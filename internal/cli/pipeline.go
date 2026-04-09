package cli

import (
	"fmt"
	"os"

	"github.com/ohare93/juggle/internal/pipeline"
	"github.com/spf13/cobra"
)

var pipelineFile string

var pipelineCmd = &cobra.Command{
	Use:   "pipeline [--file <path>] [<node>...]",
	Short: "Run an agent pipeline defined as ordered agent and cmd nodes",
	Long: `Run a pipeline of agent and cmd nodes with lifecycle events, dependencies,
and failure policies.

Load a pipeline from a TOML file:

  juggle pipeline --file pipeline.toml

Or define nodes inline:

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
	pipelineCmd.Flags().StringVarP(&pipelineFile, "file", "f", "", "load pipeline from a TOML file")
	pipelineCmd.SetHelpFunc(groupedHelp)
	rootCmd.AddCommand(pipelineCmd)
}

func runPipelineCmd(cmd *cobra.Command, args []string) error {
	var p *pipeline.Pipeline
	var err error

	if pipelineFile != "" {
		p, err = pipeline.LoadFile(pipelineFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "juggle pipeline: %v\n", err)
			return err
		}
	} else {
		p, err = pipeline.ParseArgs(args)
		if err != nil {
			fmt.Fprintf(os.Stderr, "juggle pipeline: %v\n", err)
			return err
		}
	}

	if err := pipeline.Normalize(p); err != nil {
		fmt.Fprintf(os.Stderr, "juggle pipeline: %v\n", err)
		return err
	}

	fmt.Fprintf(os.Stderr, "juggle pipeline: parsed %d node(s); execution not yet implemented\n", len(p.Nodes))
	return nil
}
