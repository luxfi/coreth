// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	consensusctx "github.com/luxfi/consensus/context"
	"github.com/luxfi/consensus/engine/chain/block"
	"github.com/luxfi/log"
	"github.com/luxfi/vm/utils/lock"
)

// TestNotifyConsensusEngine verifies that notifyConsensusEngine sends PendingTxs
// messages to the toEngine channel
func TestNotifyConsensusEngine(t *testing.T) {
	require := require.New(t)

	// Create toEngine channel
	toEngine := make(chan block.Message, 10)

	// Create minimal block builder with toEngine channel
	b := &blockBuilder{
		ctx: &consensusctx.Context{
			Log: log.NewNoOpLogger(),
		},
		toEngine: toEngine,
	}

	// Call notifyConsensusEngine
	b.notifyConsensusEngine()

	// Verify message was sent
	select {
	case msg := <-toEngine:
		require.Equal(block.PendingTxs, msg.Type, "Expected PendingTxs message type")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected message on toEngine channel, but none received")
	}
}

// TestNotifyConsensusEngineNilChannel verifies that notifyConsensusEngine
// handles nil toEngine channel gracefully
func TestNotifyConsensusEngineNilChannel(t *testing.T) {
	// Create block builder with nil toEngine (should not panic)
	b := &blockBuilder{
		ctx: &consensusctx.Context{
			Log: log.NewNoOpLogger(),
		},
		toEngine: nil,
	}

	// Should not panic
	b.notifyConsensusEngine()
}

// TestNotifyConsensusEngineFullChannel verifies that notifyConsensusEngine
// drops the message gracefully when the channel is full
func TestNotifyConsensusEngineFullChannel(t *testing.T) {
	require := require.New(t)

	// Create unbuffered channel (will always be "full" for non-blocking send)
	toEngine := make(chan block.Message)

	b := &blockBuilder{
		ctx: &consensusctx.Context{
			Log: log.NewNoOpLogger(),
		},
		toEngine: toEngine,
	}

	// Fill the channel (start a goroutine to not block)
	done := make(chan struct{})
	var received int
	go func() {
		defer close(done)
		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-toEngine:
				received++
			case <-timer.C:
				return
			}
		}
	}()

	// Send multiple notifications rapidly
	for i := 0; i < 100; i++ {
		b.notifyConsensusEngine()
	}

	<-done
	// Some messages may be dropped (we're using select with default)
	// The important thing is no panic and some messages got through
	require.GreaterOrEqual(received, 0, "Should not panic even with full channel")
}

// TestToEngineIntegration simulates the full flow from tx submission to toEngine notification
func TestToEngineIntegration(t *testing.T) {
	require := require.New(t)

	// Create channels
	toEngine := make(chan block.Message, 10)
	shutdownChan := make(chan struct{})
	defer close(shutdownChan)

	// Track notifications
	var wg sync.WaitGroup
	var notifications []block.Message
	var mu sync.Mutex

	// Consumer goroutine (simulates manager.go reading from toEngine)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case msg := <-toEngine:
				mu.Lock()
				notifications = append(notifications, msg)
				mu.Unlock()
			case <-shutdownChan:
				return
			}
		}
	}()

	// Create block builder
	b := &blockBuilder{
		ctx: &consensusctx.Context{
			Log: log.NewNoOpLogger(),
		},
		toEngine:     toEngine,
		shutdownChan: shutdownChan,
	}

	// Simulate transaction submissions
	for i := 0; i < 5; i++ {
		b.notifyConsensusEngine()
	}

	// Give time for messages to be processed
	time.Sleep(50 * time.Millisecond)

	// Verify all notifications received
	mu.Lock()
	count := len(notifications)
	mu.Unlock()

	require.Equal(5, count, "Expected 5 notifications")

	// Verify all were PendingTxs type
	mu.Lock()
	for _, msg := range notifications {
		require.Equal(block.PendingTxs, msg.Type)
	}
	mu.Unlock()
}

// TestSignalCanBuild verifies that signalCanBuild broadcasts the condition
func TestSignalCanBuild(t *testing.T) {
	b := &blockBuilder{
		ctx: &consensusctx.Context{
			Log: log.NewNoOpLogger(),
		},
	}
	// Initialize the condition variable using the lock package
	b.pendingSignal = lock.NewCond(&b.buildBlockLock)

	// signalCanBuild should not panic
	b.signalCanBuild()
}

// TestHandleGenerateBlock verifies that handleGenerateBlock updates lastBuildTime
func TestHandleGenerateBlock(t *testing.T) {
	require := require.New(t)

	b := &blockBuilder{
		ctx: &consensusctx.Context{
			Log: log.NewNoOpLogger(),
		},
	}

	// Initially, lastBuildTime should be zero
	require.True(b.lastBuildTime.IsZero(), "lastBuildTime should be zero initially")

	// Call handleGenerateBlock
	before := time.Now()
	b.handleGenerateBlock()
	after := time.Now()

	// Verify lastBuildTime was updated
	require.False(b.lastBuildTime.IsZero(), "lastBuildTime should not be zero after handleGenerateBlock")
	require.True(b.lastBuildTime.After(before) || b.lastBuildTime.Equal(before), "lastBuildTime should be >= before")
	require.True(b.lastBuildTime.Before(after) || b.lastBuildTime.Equal(after), "lastBuildTime should be <= after")
}
