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
	assert.True(t, metrics.Enabled, "geth/metrics.Enabled")

	switch m := metrics.NewCounter().(type) {
	case metrics.NilCounter:
		t.Errorf("metrics.NewCounter() got %T; want %T", m, new(metrics.StandardCounter))
	case *metrics.StandardCounter:
	default:
		t.Errorf("metrics.NewCounter() got unknown type %T", m)
	}
}
