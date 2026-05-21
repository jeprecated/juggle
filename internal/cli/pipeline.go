package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jeprecated/juggle/internal/agent"
	"github.com/jeprecated/juggle/internal/agent/provider"
	"github.com/jeprecated/juggle/internal/pipeline"
	"github.com/spf13/cobra"
)

var pipelineFile string

// pipelineTestRunner is injectable for tests (nil = build from flags).
var pipelineTestRunner agent.Runner

var pipelineCmd = &cobra.Command{
	Use:   "pipeline [--file <path>] [<node>...]",
	Short: "Run an agent pipeline defined as ordered agent and cmd nodes",
	Long: `Run a pipeline of agent and cmd nodes with lifecycle events, dependencies,
and failure policies.

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
	Example: `  # Load a pipeline from a TOML file
  juggle pipeline --file pipeline.toml

  # Define nodes inline
  juggle pipeline \
    agent "Setup" @setup.md --event run-start --model haiku \
    agent "Implement" @task.md --event loop-body \
    cmd "Commit" "git add -A && git commit -m done" --event loop-end`,
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

	if pipelineFile != "" && len(args) > 0 {
		return fmt.Errorf("cannot use --file with inline node arguments")
	}

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

	runner, err := resolvePipelineRunner()
	if err != nil {
		return err
	}

	workdir := flags.workdir
	if workdir == "" {
		if cwd, cwdErr := os.Getwd(); cwdErr == nil {
			workdir = cwd
		}
	}

	shutdown := make(chan struct{})
	var shutdownOnce sync.Once
	forceCtx, forceCancel := context.WithCancel(context.Background())
	defer forceCancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	go func() {
		<-sigCh
		shutdownOnce.Do(func() { close(shutdown) })
		<-sigCh
		forceCancel()
		forceCleanupSession()
		time.Sleep(200 * time.Millisecond)
		os.Exit(130)
	}()

	sessionType := "pipeline"
	if flags.id != "" {
		eid := EffectiveID(flags.id, workdir)
		info := SessionInfo{
			PID:       os.Getpid(),
			Type:      sessionType,
			WorkDir:   workdir,
			StartedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := RegisterSession(eid, info); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "session %q registered\n", eid)
			activeSessionID = eid
			wc := NewWakeChecker(eid)
			go wc.Run()
			defer func() {
				wc.Stop()
				UnregisterSession(eid)
				activeSessionID = ""
			}()

			return executePipeline(p, pipeline.ExecutorConfig{
				Runner:        runner,
				RunnerFactory: makeRunnerFactory(flags.agentCmd),
				Stdout:        os.Stdout,
				Stderr:        os.Stderr,
				ForceCtx:      forceCtx,
				Shutdown:      shutdown,
				WorkDir:       workdir,
				RunID:         generateRunID(),
				SessionID:     eid,
				WakeCh:        wc.WakeCh,
				ReadTrigger:   ReadTrigger,
				FormatTrigger: FormatTrigger,
			})
		}
	}

	return executePipeline(p, pipeline.ExecutorConfig{
		Runner:        runner,
		RunnerFactory: makeRunnerFactory(flags.agentCmd),
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		ForceCtx:      forceCtx,
		Shutdown:      shutdown,
		WorkDir:       workdir,
		RunID:         generateRunID(),
	})
}

// resolvePipelineRunner returns the runner to use for pipeline execution.
// In tests, pipelineTestRunner can be set to bypass provider flag resolution.
func resolvePipelineRunner() (agent.Runner, error) {
	if pipelineTestRunner != nil {
		return pipelineTestRunner, nil
	}
	providerName := flags.provider
	agentCmd := flags.agentCmd
	if agentCmd != "" && providerName == "claude" {
		providerName = "custom"
	}
	if provider.Type(providerName) == provider.TypeCustom {
		if agentCmd == "" {
			return nil, fmt.Errorf("--provider custom requires --agent-cmd")
		}
		return &agent.ProviderRunner{Provider: provider.GetCustom(agentCmd)}, nil
	}
	return &agent.ProviderRunner{Provider: provider.Get(provider.Type(providerName))}, nil
}

// executePipeline runs the pipeline with the given executor config.
func executePipeline(p *pipeline.Pipeline, cfg pipeline.ExecutorConfig) error {
	return pipeline.NewExecutor(p, cfg).Run()
}
