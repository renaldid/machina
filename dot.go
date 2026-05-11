package machina

import (
	"fmt"
	"strings"
)

// DOT returns a Graphviz DOT representation of the FSM's transition graph.
// Render with: dot -Tpng graph.dot -o graph.png
func (f *FSM[S, E]) DOT() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var b strings.Builder
	b.WriteString("digraph machina {\n\trankdir=LR;\n")
	for from, events := range f.table {
		for event, tr := range events {
			fmt.Fprintf(&b, "\t%q -> %q [label=%q];\n",
				fmt.Sprint(from), fmt.Sprint(tr.to), fmt.Sprint(event))
		}
	}
	b.WriteString("}")
	return b.String()
}
