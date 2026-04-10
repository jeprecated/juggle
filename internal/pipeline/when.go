package pipeline

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// WhenContext carries the state available to when-condition expressions.
type WhenContext struct {
	Iteration    int
	PrevSuccess  bool
	PrevExitCode int
}

var (
	reIteration = regexp.MustCompile(`^iteration\s*(==|!=|>=|<=|>|<)\s*(\d+)$`)
	reSuccess   = regexp.MustCompile(`^success\s*==\s*(true|false)$`)
	reExitCode  = regexp.MustCompile(`^exit_code\s*==\s*(\d+)$`)
)

// EvalStructured tries to evaluate expr as a structured when condition in-process.
// Returns (result, true, nil) when the expression matched the grammar and was evaluated.
// Returns (false, false, nil) when the expression doesn't look structured (caller should fall back to shell).
// Returns (false, false, err) when the expression looks like a structured condition but is malformed.
func EvalStructured(expr string, wctx WhenContext) (bool, bool, error) {
	trimmed := strings.TrimSpace(expr)

	if m := reIteration.FindStringSubmatch(trimmed); m != nil {
		n, _ := strconv.Atoi(m[2])
		return evalIterationOp(wctx.Iteration, m[1], n), true, nil
	}

	if m := reSuccess.FindStringSubmatch(trimmed); m != nil {
		want := m[1] == "true"
		return wctx.PrevSuccess == want, true, nil
	}

	if m := reExitCode.FindStringSubmatch(trimmed); m != nil {
		n, _ := strconv.Atoi(m[1])
		return wctx.PrevExitCode == n, true, nil
	}

	if looksStructured(trimmed) {
		return false, false, fmt.Errorf("invalid structured when condition: %q", trimmed)
	}

	return false, false, nil
}

func evalIterationOp(lhs int, op string, rhs int) bool {
	switch op {
	case "==":
		return lhs == rhs
	case "!=":
		return lhs != rhs
	case ">":
		return lhs > rhs
	case ">=":
		return lhs >= rhs
	case "<":
		return lhs < rhs
	case "<=":
		return lhs <= rhs
	}
	return false
}

// looksStructured returns true when expr appears to be a structured when condition
// (starts with a known keyword followed by an operator or whitespace).
func looksStructured(expr string) bool {
	for _, prefix := range []string{"iteration", "success", "exit_code"} {
		if !strings.HasPrefix(expr, prefix) {
			continue
		}
		rest := expr[len(prefix):]
		if len(rest) == 0 {
			continue
		}
		switch rest[0] {
		case '=', '!', '<', '>', ' ', '\t':
			return true
		}
	}
	return false
}
