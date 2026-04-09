package pipeline

import "fmt"

// Normalize inserts implicit dependencies into p and validates the result.
// It mutates p.Nodes in place, setting After on nodes that have no explicit
// After and are not marked Parallel. It returns the first validation error.
func Normalize(p *Pipeline) error {
	if err := validateUniqueNames(p); err != nil {
		return err
	}
	insertImplicitDeps(p)
	if err := validateKindPayloads(p); err != nil {
		return err
	}
	if err := validateEvents(p); err != nil {
		return err
	}
	if err := validateAfterTargets(p); err != nil {
		return err
	}
	if err := detectCycles(p); err != nil {
		return err
	}
	return validateLoopBodyInvariant(p)
}

// insertImplicitDeps sets After on each node that has no explicit After and is
// not marked Parallel, making it implicitly depend on the previous node.
func insertImplicitDeps(p *Pipeline) {
	for i := range p.Nodes {
		if i == 0 {
			continue
		}
		n := &p.Nodes[i]
		if len(n.After) == 0 && !n.Parallel {
			n.After = []string{p.Nodes[i-1].Name}
		}
	}
}

func validateUniqueNames(p *Pipeline) error {
	seen := make(map[string]struct{}, len(p.Nodes))
	for _, n := range p.Nodes {
		if _, exists := seen[n.Name]; exists {
			return fmt.Errorf("duplicate node name %q", n.Name)
		}
		seen[n.Name] = struct{}{}
	}
	return nil
}

func validateKindPayloads(p *Pipeline) error {
	for _, n := range p.Nodes {
		switch n.Kind {
		case NodeKindAgent:
			if n.Agent == nil {
				return fmt.Errorf("node %q has kind %q but no agent spec", n.Name, n.Kind)
			}
		case NodeKindCmd:
			if n.Cmd == nil {
				return fmt.Errorf("node %q has kind %q but no cmd spec", n.Name, n.Kind)
			}
		}
	}
	return nil
}

func validateEvents(p *Pipeline) error {
	for _, n := range p.Nodes {
		if n.Event != "" && !n.Event.Valid() {
			return fmt.Errorf("node %q has invalid event %q", n.Name, n.Event)
		}
	}
	return nil
}

func validateAfterTargets(p *Pipeline) error {
	indexByName := make(map[string]int, len(p.Nodes))
	for i, n := range p.Nodes {
		indexByName[n.Name] = i
	}
	for i, n := range p.Nodes {
		for _, dep := range n.After {
			depIdx, ok := indexByName[dep]
			if !ok {
				return fmt.Errorf("node %q depends on unknown node %q", n.Name, dep)
			}
			if depIdx >= i {
				return fmt.Errorf("node %q has forward dependency on %q", n.Name, dep)
			}
		}
	}
	return nil
}

// detectCycles uses DFS to find cycles in the dependency graph. In practice,
// validateAfterTargets already prevents forward references which makes cycles
// impossible, but this check provides a belt-and-suspenders guarantee.
func detectCycles(p *Pipeline) error {
	deps := make(map[string][]string, len(p.Nodes))
	for _, n := range p.Nodes {
		deps[n.Name] = n.After
	}

	const (
		unvisited = 0
		inStack   = 1
		done      = 2
	)
	state := make(map[string]int, len(p.Nodes))

	var dfs func(name string) error
	dfs = func(name string) error {
		state[name] = inStack
		for _, dep := range deps[name] {
			if state[dep] == inStack {
				return fmt.Errorf("dependency cycle detected involving node %q", name)
			}
			if state[dep] == unvisited {
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		state[name] = done
		return nil
	}

	for _, n := range p.Nodes {
		if state[n.Name] == unvisited {
			if err := dfs(n.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateLoopBodyInvariant(p *Pipeline) error {
	count := 0
	for _, n := range p.Nodes {
		if n.Kind == NodeKindAgent && n.Event == EventLoopBody {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("pipeline must have exactly one agent node with event=loop-body; got %d", count)
	}
	return nil
}
