// Copyright 2019 spaGO Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ag

import (
	"github.com/nlpodyssey/spago/pkg/mat"
	"sync"
)

type forwardHandler struct {
	g            *Graph
	fromTimeStep int64 // default 0
	toTimeStep   int64 // default -1 (no limit)
}

func (h *forwardHandler) runSerial() {
	for _, node := range h.g.nodes {
		if op, ok := node.(*operator); ok {
			if op.timeStep < h.fromTimeStep {
				continue
			}
			if h.toTimeStep != -1 && op.timeStep > h.toTimeStep {
				continue
			}
			op.value = op.function.Forward()
		}
	}
}

func (h *forwardHandler) runConcurrent() {
	groups := h.g.groupNodesByHeight()
	var wg sync.WaitGroup
	for _, group := range groups {
		for _, node := range group {
			if op, ok := node.(*operator); ok {
				if op.timeStep < h.fromTimeStep {
					continue
				}
				if h.toTimeStep != -1 && op.timeStep > h.toTimeStep {
					continue
				}
				wg.Add(1)
				go func(op *operator) {
					defer wg.Done()
					op.value = op.function.Forward()
				}(op)
			}
		}
		wg.Wait()
	}
}

type backwardHandler struct {
	g              *Graph
	node           Node
	outputGrad     mat.Matrix
	stopAtTimeStep int64 // default -1 (full backward)
}

func (h *backwardHandler) propagateOutputGrad() {
	gx := h.outputGrad
	if gx == nil {
		gx = h.node.Value().OnesLike()
		defer mat.ReleaseDense(gx.(*mat.Dense))
	}
	h.node.PropagateGrad(gx)
}

func (h *backwardHandler) runSerial() {
	nodes := h.g.nodes
	lastIndex := h.node.Id()
	stopAtTimeStep := h.stopAtTimeStep
	truncated := stopAtTimeStep > -1
	_ = nodes[lastIndex] // avoid bounds check
	for i := lastIndex; i >= 0; i-- {
		if truncated && nodes[i].getTimeStep() <= stopAtTimeStep {
			break
		}
		if node, ok := nodes[i].(*operator); ok {
			node.backward()
		}
	}
}

func (h *backwardHandler) runConcurrent() {
	stopAtTimeStep := h.stopAtTimeStep
	truncated := stopAtTimeStep > -1
	groups := h.g.groupNodesByHeight()
	lastGroupIndex := h.g.cache.height[h.node.Id()]
	lastNodeIndex := h.node.Id()
	var wg sync.WaitGroup
	for i := lastGroupIndex; i >= 0; i-- {
		for _, node := range groups[i] {
			if truncated && node.getTimeStep() <= stopAtTimeStep {
				break
			}
			if op, ok := node.(*operator); ok {
				if op.id > lastNodeIndex {
					continue
				}
				wg.Add(1)
				go func(op *operator) {
					defer wg.Done()
					op.backward()
				}(op)
			}
		}
		wg.Wait()
	}
}
