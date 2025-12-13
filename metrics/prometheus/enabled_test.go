// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package prometheus_test

import (
	"testing"

	// NOTE: This test assumes that there are no imported packages that might
	// change the default value of [metrics.Enabled]. It is therefore in package
	// `prometheus_test` in case any other tests modify the variable. If any
	// imports here or in the implementation do actually do so then this test
	// may have false negatives.
	"github.com/stretchr/testify/assert"

	"github.com/luxfi/geth/metrics"
)

func TestMetricsEnabledByDefault(t *testing.T) {
	assert.True(t, metrics.Enabled(), "geth/metrics.Enabled")

	// Verify that a counter can be created (metrics are enabled)
	counter := metrics.NewCounter()
	assert.NotNil(t, counter, "metrics.NewCounter() should not return nil")

	// Test that the counter works
	counter.Inc(1)
	assert.Equal(t, int64(1), counter.Snapshot().Count(), "counter should have value 1 after Inc(1)")
}
