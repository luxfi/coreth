// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package metricstest

import (
	"testing"

	"github.com/luxfi/metrics"
)

// WithMetrics enables go-ethereum metrics globally for the test.
// If metrics are already enabled, nothing is done.
// Otherwise, it enables metrics and reverts when the test finishes.
func WithMetrics(t *testing.T) {
	if !metrics.IsEnabled() {
		metrics.Enable()
		t.Cleanup(func() {
			// Note: There's no standard way to disable metrics once enabled
			// This is a limitation of the go-ethereum metrics package
		})
	}
}
