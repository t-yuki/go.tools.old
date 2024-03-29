// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pointer

// This file defines the constraint optimiser ("pre-solver").

import (
	"fmt"
)

func (a *analysis) optimize() {
	a.renumber()

	// TODO(adonovan): opt: PE (HVN, HRU), LE, etc.
}

// renumber permutes a.nodes so that all nodes within an addressable
// object appear before all non-addressable nodes, maintaining the
// order of nodes within the same object (as required by offsetAddr).
//
// renumber must update every nodeid in the analysis (constraints,
// Pointers, callgraph, etc) to reflect the new ordering.
//
// This is an optimisation to increase the locality and efficiency of
// sparse representations of points-to sets.  (Typically only about
// 20% of nodes are within an object.)
//
// NB: nodes added during solving (e.g. for reflection, SetFinalizer)
// will be appended to the end.
//
func (a *analysis) renumber() {
	N := nodeid(len(a.nodes))
	newNodes := make([]*node, N, N)
	renumbering := make([]nodeid, N, N) // maps old to new

	var i, j nodeid

	// The zero node is special.
	newNodes[j] = a.nodes[i]
	renumbering[i] = j
	i++
	j++

	// Pass 1: object nodes.
	for i < N {
		obj := a.nodes[i].obj
		if obj == nil {
			i++
			continue
		}

		end := i + nodeid(obj.size)
		for i < end {
			newNodes[j] = a.nodes[i]
			renumbering[i] = j
			i++
			j++
		}
	}
	nobj := j

	// Pass 2: non-object nodes.
	for i = 1; i < N; {
		obj := a.nodes[i].obj
		if obj != nil {
			i += nodeid(obj.size)
			continue
		}

		newNodes[j] = a.nodes[i]
		renumbering[i] = j
		i++
		j++
	}

	if j != N {
		panic(fmt.Sprintf("internal error: j=%d, N=%d", j, N))
	}

	// Log the remapping table.
	if a.log != nil {
		fmt.Fprintf(a.log, "Renumbering nodes to improve density:\n")
		fmt.Fprintf(a.log, "(%d object nodes of %d total)\n", nobj, N)
		for old, new := range renumbering {
			fmt.Fprintf(a.log, "\tn%d -> n%d\n", old, new)
		}
	}

	// Now renumber all existing nodeids to use the new node permutation.
	// It is critical that all reachable nodeids are accounted for!

	// Renumber nodeids in queried Pointers.
	for v, ptr := range a.result.Queries {
		ptr.n = renumbering[ptr.n]
		a.result.Queries[v] = ptr
	}
	for v, ptr := range a.result.IndirectQueries {
		ptr.n = renumbering[ptr.n]
		a.result.IndirectQueries[v] = ptr
	}

	// Renumber nodeids in global objects.
	for v, id := range a.globalobj {
		a.globalobj[v] = renumbering[id]
	}

	// Renumber nodeids in constraints.
	for _, c := range a.constraints {
		c.renumber(renumbering)
	}

	// Renumber nodeids in the call graph.
	for _, cgn := range a.cgnodes {
		cgn.obj = renumbering[cgn.obj]
		for _, site := range cgn.sites {
			site.targets = renumbering[site.targets]
		}
	}

	a.nodes = newNodes
}
