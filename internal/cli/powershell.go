package cli

import (
	"io"

	"github.com/spf13/cobra"
)

// genPowerShellCompletion writes a PowerShell completion script for the given command tree to w.
func genPowerShellCompletion(root *cobra.Command, w io.Writer) {
	root.GenPowerShellCompletion(w) //nolint:errcheck
}
