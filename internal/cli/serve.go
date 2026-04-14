package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ohare93/juggle/internal/agent"
	"github.com/ohare93/juggle/internal/agent/provider"
	"github.com/spf13/cobra"
)

// serveSpecificFlags holds flags only applicable to the serve subcommand.
var serveSpecificFlags struct {
	port int
	bind string
}

// generateServeFilename returns a filename of the form YYYYMMDD-HHMMSS-<id><ext>.
func generateServeFilename(ts time.Time, id string, ext string) string {
	return fmt.Sprintf("%s-%s-%s%s",
		ts.Format("20060102"),
		ts.Format("150405"),
		id,
		ext,
	)
}

// newServeHandler returns an HTTP handler that accepts POST requests and writes
// the request body to a file in dir. The URL path must have a .md, .txt, or
// .json extension; other paths return 404. Non-POST methods return 405.
func newServeHandler(dir string) http.Handler {
	allowed := map[string]bool{".md": true, ".txt": true, ".json": true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		ext := filepath.Ext(r.URL.Path)
		if !allowed[ext] {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := generateRunID()
		if len(id) > 8 {
			id = id[:8]
		}
		name := generateServeFilename(time.Now(), id, ext)
		body, _ := io.ReadAll(r.Body)
		_ = os.WriteFile(filepath.Join(dir, name), body, 0644)
		w.WriteHeader(http.StatusAccepted)
	})
}

// RunServe starts an HTTP server alongside RunWatch.
// RunWatch runs in a goroutine; the HTTP server runs in the calling goroutine.
// Both stop when cfg.Shutdown is closed.
func RunServe(cfg Config, addr string) error {
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	watchDir := cfg.Watch[0]
	server := &http.Server{
		Addr:    addr,
		Handler: newServeHandler(watchDir),
	}

	watchErr := make(chan error, 1)
	go func() {
		watchErr <- RunWatch(cfg)
	}()

	if cfg.Shutdown != nil {
		go func() {
			<-cfg.Shutdown
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = server.Shutdown(shutCtx)
		}()
	}

	fmt.Fprintf(cfg.Stderr, "juggle serve: listening on http://%s (watching %s)\n", addr, watchDir)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return <-watchErr
}

var serveCmd = &cobra.Command{
	Use:   "serve <folder> [prompt-content...]",
	Short: "Start HTTP API + watch on a folder",
	Long: `Start an HTTP server and watch a folder for task files.

POST a prompt to the server to write a task file into the folder.
The built-in watch picks it up and runs it as an agent session.

Supported endpoints:
  POST /prompt.txt  — write a .txt task file
  POST /prompt.md   — write a .md task file
  POST /prompt.json — write a .json task file

All requests return 202 Accepted with an empty body.`,
	Args:          cobra.MinimumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServeCmd(cmd, args)
	},
}

func init() {
	f := serveCmd.Flags()

	// Serve-specific flags
	f.IntVar(&serveSpecificFlags.port, "port", 8080, "port to listen on")
	f.StringVar(&serveSpecificFlags.bind, "bind", "127.0.0.1", "address to bind to")

	// Watch-mode flags (shared with watchCmd via watchFlags)
	f.IntVar(&watchFlags.workers, "workers", 1, "number of concurrent watch workers")
	f.BoolVar(&watchFlags.dashboard, "dashboard", false, "show TUI dashboard for watch workers")

	for _, name := range []string{"workers", "dashboard"} {
		setFlagGroup(f, name, "Watch Mode")
	}
	for _, name := range []string{"port", "bind"} {
		setFlagGroup(f, name, "Serve")
	}

	serveCmd.SetHelpFunc(groupedHelp)
	rootCmd.AddCommand(serveCmd)
}

func runServeCmd(cmd *cobra.Command, args []string) error {
	watchDir := args[0]
	promptArgs := args[1:]

	fileCfg, _, cfgErr := LoadConfig(flags.noConfig, ".", os.Stderr)
	if cfgErr != nil {
		return cfgErr
	}
	if fileCfg != nil {
		if fileCfg.Verbose != nil && !cmd.Flags().Changed("verbose") {
			flags.verbose = *fileCfg.Verbose
		}
		ApplyFileConfig(fileCfg, cmd.Flags().Changed, flags.verbose, os.Stderr)
	}

	resolved, err := ResolveArgs(promptArgs)
	if err != nil {
		return err
	}

	agentPre, err := BuildPhaseContent(flags.agentPre)
	if err != nil {
		return fmt.Errorf("--agent-pre: %w", err)
	}
	agentBefore, err := BuildPhaseContent(flags.agentBefore)
	if err != nil {
		return fmt.Errorf("--agent-before: %w", err)
	}
	agentAfter, err := BuildPhaseContent(flags.agentAfter)
	if err != nil {
		return fmt.Errorf("--agent-after: %w", err)
	}
	agentPost, err := BuildPhaseContent(flags.agentPost)
	if err != nil {
		return fmt.Errorf("--agent-post: %w", err)
	}

	var systemPrompt string
	if flags.systemPrompt != "" {
		r1, err1 := ResolveArgs([]string{flags.systemPrompt})
		if err1 != nil {
			return fmt.Errorf("--system-prompt: %w", err1)
		}
		systemPrompt = r1[0]
	}

	if flags.mcpConfig != "" {
		if _, err := os.Stat(flags.mcpConfig); err != nil {
			return fmt.Errorf("--mcp-config: file not found: %s", flags.mcpConfig)
		}
	}

	hooksSettingsFile, hooksCleanup, err := buildHooksSettingsFile(flags.hooks, flags.hooksFile)
	if err != nil {
		return fmt.Errorf("hooks: %w", err)
	}
	defer hooksCleanup()

	if flags.agentCmd != "" && flags.provider == "claude" {
		flags.provider = "custom"
	}

	var p provider.Provider
	if provider.Type(flags.provider) == provider.TypeCustom {
		if flags.agentCmd == "" {
			return fmt.Errorf("--provider custom requires --agent-cmd")
		}
		p = provider.GetCustom(flags.agentCmd)
	} else {
		p = provider.Get(provider.Type(flags.provider))
	}

	cfg := Config{
		Content:           strings.Join(resolved, "\n\n"),
		Watch:             []string{watchDir},
		Iterations:        flags.iterations,
		Model:             flags.model,
		Provider:          flags.provider,
		Delay:             flags.delay,
		Trust:             flags.trust,
		Plan:              flags.plan,
		Timeout:           flags.timeout,
		MaxWait:           flags.maxWait,
		Verbose:           flags.verbose,
		MaxFailures:       flags.maxFailures,
		CmdBefore:         flags.cmdBefore,
		CmdAfter:          flags.cmdAfter,
		StopWhen:          flags.stopWhen,
		AgentPre:          agentPre,
		AgentBefore:       agentBefore,
		AgentAfter:        agentAfter,
		AgentPost:         agentPost,
		Hooks:             flags.hooks,
		HooksSettingsFile: hooksSettingsFile,
		Log:               flags.log,
		MaxCost:           flags.maxCost,
		Label:             flags.label,
		AllowedTools:      flags.allowedTools,
		DisallowedTools:   flags.disallowedTools,
		MaxTurns:          flags.maxTurns,
		MCPConfig:         flags.mcpConfig,
		OnFailure:         OnFailure(flags.onFailure),
		Retries:           flags.retries,
		AgentCmd:          flags.agentCmd,
		SystemPrompt:      systemPrompt,
		Workers:           watchFlags.workers,
		WorkDir:           flags.workdir,
		Dashboard:         watchFlags.dashboard,
		Runner:            &agent.ProviderRunner{Provider: p},
		ID:                flags.id,
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
		time.Sleep(200 * time.Millisecond)
		os.Exit(130)
	}()

	cfg.Shutdown = shutdown
	cfg.ForceCtx = forceCtx

	setupSession(&cfg, cfg.Stderr, "watch")
	defer teardownSession(&cfg)

	if tty, ttyCleanup, ttyErr := openTTYKeypress(); ttyErr == nil {
		color := isColorEnabled(cfg.Stderr)
		trigger := func() { shutdownOnce.Do(func() { close(shutdown) }) }
		_ = StartKeypressListener(tty, trigger, color, cfg.Stderr)
		defer ttyCleanup()
	}

	addr := fmt.Sprintf("%s:%d", serveSpecificFlags.bind, serveSpecificFlags.port)
	return RunServe(cfg, addr)
}
