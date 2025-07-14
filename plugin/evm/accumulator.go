// (c) 2021-2025, Lux Industries Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import "sync"

// Accumulator is a simple interface for accumulating gossip messages
type Accumulator[T any] interface {
	Add(T)
	Gossip() []T
	Size() int
}

// SimpleAccumulator implements Accumulator
type SimpleAccumulator[T any] struct {
	items []T
	mu    sync.Mutex
}

func NewAccumulator[T any]() Accumulator[T] {
	return &SimpleAccumulator[T]{
		items: make([]T, 0),
	}
}

func (a *SimpleAccumulator[T]) Add(item T) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.items = append(a.items, item)
}

func (a *SimpleAccumulator[T]) Gossip() []T {
	a.mu.Lock()
	defer a.mu.Unlock()
	items := a.items
	a.items = make([]T, 0)
	return items
}

func (a *SimpleAccumulator[T]) Size() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.items)
}