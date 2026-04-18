package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagGroupKey is the pflag annotation key used to assign a flag to a help group.
const flagGroupKey = "group"

// excludeFlagsKey is the cobra command annotation key for comma-separated flag
// names that should be suppressed from inherited help output.
const excludeFlagsKey = "exclude-flags"

// groupOrder defines the display order of flag groups in --help output.
var groupOrder = []string{
	"Loop Control",
	"Queue Mode",
	"Agent Configuration",
	"Lifecycle Hooks",
	"Output",
}

// setFlagGroup annotates a flag with its help group.
// Panics if the flag name is not registered, to catch typos at startup.
func setFlagGroup(f *pflag.FlagSet, name, group string) {
	fl := f.Lookup(name)
	if fl == nil {
		panic("setFlagGroup: unknown flag " + name)
	}
	if fl.Annotations == nil {
		fl.Annotations = make(map[string][]string)
	}
	fl.Annotations[flagGroupKey] = []string{group}
}

func parseExcludeFlags(cmd *cobra.Command) map[string]bool {
	excluded := make(map[string]bool)
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		excluded[f.Name] = true
	})
	if v, ok := cmd.Annotations[excludeFlagsKey]; ok && v != "" {
		for _, name := range strings.Split(v, ",") {
			excluded[strings.TrimSpace(name)] = true
		}
	}
	return excluded
}

// parseAnnotatedExclusions returns only the flag names from the command's
// exclude-flags annotation (not local flags). Used to suppress inherited
// flags globally for commands that don't need them.
func parseAnnotatedExclusions(cmd *cobra.Command) map[string]bool {
	excluded := make(map[string]bool)
	if v, ok := cmd.Annotations[excludeFlagsKey]; ok && v != "" {
		for _, name := range strings.Split(v, ",") {
			excluded[strings.TrimSpace(name)] = true
		}
	}
	return excluded
}

// groupedHelp is a cobra help function that renders flags sorted into named groups.
// Color is enabled when the command's output writer is a TTY and NO_COLOR is unset.
func groupedHelp(cmd *cobra.Command, args []string) {
	groupedHelpWithColor(cmd, args, isColorEnabled(cmd.OutOrStdout()))
}

// groupedHelpWithColor renders grouped help output; color controls ANSI styling.
func groupedHelpWithColor(cmd *cobra.Command, _ []string, color bool) {
	w := cmd.OutOrStdout()

	if cmd.Long != "" {
		fmt.Fprintln(w, cmd.Long)
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\n  %s\n\n", colorizeHeading("Usage:", color), cmd.UseLine())

	if cmd.Example != "" {
		fmt.Fprintf(w, "%s\n%s\n\n", colorizeHeading("Examples:", color), colorizeExamples(cmd.Example, color))
	}

	// Compute exclusions: annotated exclusions suppress inherited flags globally;
	// full exclusions (including local flags) prevent duplicates from InheritedFlags.
	annotated := parseAnnotatedExclusions(cmd)
	excluded := parseExcludeFlags(cmd)

	// Collect flags into groups.
	// Visit both cmd.Flags() (local + inherited persistent) and cmd.PersistentFlags()
	// (the command's own persistent flags, not visible in Flags() for root commands).
	groupSets := make(map[string]*pflag.FlagSet)
	otherSet := pflag.NewFlagSet("other", pflag.ContinueOnError)
	seen := make(map[string]bool)

	addFlag := func(f *pflag.Flag) {
		if seen[f.Name] || f.Hidden || annotated[f.Name] {
			return
		}
		seen[f.Name] = true
		if grps, ok := f.Annotations[flagGroupKey]; ok && len(grps) > 0 {
			grp := grps[0]
			if groupSets[grp] == nil {
				groupSets[grp] = pflag.NewFlagSet(grp, pflag.ContinueOnError)
			}
			groupSets[grp].AddFlag(f)
		} else {
			otherSet.AddFlag(f)
		}
	}

	cmd.Flags().VisitAll(addFlag)
	cmd.PersistentFlags().VisitAll(addFlag)

	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if excluded[f.Name] {
			return
		}
		addFlag(f)
	})

	// Render groups in defined order
	for _, grp := range groupOrder {
		fs := groupSets[grp]
		if fs == nil {
			continue
		}
		fmt.Fprintf(w, "%s\n", colorizeHeading(grp+":", color))
		fmt.Fprint(w, colorizeFlagUsages(fs.FlagUsages(), color))
		fmt.Fprintln(w)
	}

	// Render ungrouped flags (--help, --version, --no-config)
	hasOther := false
	otherSet.VisitAll(func(_ *pflag.Flag) { hasOther = true })
	if hasOther {
		fmt.Fprintf(w, "%s\n", colorizeHeading("Flags:", color))
		fmt.Fprint(w, colorizeFlagUsages(otherSet.FlagUsages(), color))
		fmt.Fprintln(w)
	}

	// Subcommands
	available := make([]*cobra.Command, 0)
	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			available = append(available, sub)
		}
	}
	if len(available) > 0 {
		fmt.Fprintf(w, "%s\n", colorizeHeading("Available Commands:", color))
		for _, sub := range available {
			fmt.Fprintf(w, "  %-11s %s\n", sub.Name(), sub.Short)
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Use \"%s [command] --help\" for more information about a command.\n", cmd.Root().Name())
	}
}
