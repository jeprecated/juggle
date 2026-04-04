package cli

import (
	"fmt"
	"os"
	"strings"
)

// ResolveArgs processes positional arguments.
// Args starting with @ are file paths (contents read and returned).
// All other args are returned as-is.
func ResolveArgs(args []string) ([]string, error) {
	resolved := make([]string, 0, len(args))
	for _, arg := range args {
		if !strings.HasPrefix(arg, "@") && !strings.ContainsAny(arg, " \t\n") {
			return nil, fmt.Errorf("invalid argument %q: args must start with @ (file reference) or be quoted prompt text", arg)
		}
		if strings.HasPrefix(arg, "@") {
			data, err := os.ReadFile(arg[1:])
			if err != nil {
				return nil, fmt.Errorf("failed to read %s: %w", arg[1:], err)
			}
			resolved = append(resolved, string(data))
		} else {
			resolved = append(resolved, arg)
		}
	}
	return resolved, nil
}
