// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gatherer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/luxfi/coreth/metrics/metricstest"
	"github.com/luxfi/geth/metrics"
	"github.com/luxfi/metric"
)

func TestGatherer_Gather(t *testing.T) {
	metricstest.WithMetrics(t)

	registry := metrics.NewRegistry()
	register := func(t *testing.T, name string, collector any) {
		t.Helper()
		require.NoError(t, registry.Register(name, collector))
	}

	counter := metrics.NewCounter()
	counter.Inc(12345)
	register(t, "test/counter", counter)

	counterFloat64 := metrics.NewCounterFloat64()
	counterFloat64.Inc(1.1)
	register(t, "test/counter_float64", counterFloat64)

	gauge := metrics.NewGauge()
	gauge.Update(23456)
	register(t, "test/gauge", gauge)

	gaugeFloat64 := metrics.NewGaugeFloat64()
	gaugeFloat64.Update(34567.89)
	register(t, "test/gauge_float64", gaugeFloat64)

	sample := metrics.NewUniformSample(1028)
	histogram := metrics.NewHistogram(sample)
	register(t, "test/histogram", histogram)

	meter := metrics.NewMeter()
	t.Cleanup(meter.Stop)
	meter.Mark(9999999)
	register(t, "test/meter", meter)

	timer := metrics.NewTimer()
	t.Cleanup(timer.Stop)
	timer.Update(20 * time.Millisecond)
	timer.Update(21 * time.Millisecond)
	timer.Update(22 * time.Millisecond)
	timer.Update(120 * time.Millisecond)
	timer.Update(23 * time.Millisecond)
	timer.Update(24 * time.Millisecond)
	register(t, "test/timer", timer)

	resettingTimer := metrics.NewResettingTimer()
	register(t, "test/resetting_timer", resettingTimer)
	resettingTimer.Update(time.Second) // must be after register call

	emptyResettingTimer := metrics.NewResettingTimer()
	register(t, "test/empty_resetting_timer", emptyResettingTimer)

	emptyResettingTimer.Update(time.Second) // no effect because of snapshot below
	register(t, "test/empty_resetting_timer_snapshot", emptyResettingTimer.Snapshot())

	gatherer := NewGatherer(registry)

	families, err := gatherer.Gather()
	require.NoError(t, err)

	// Build expected metrics to match gatherer output format
	// The gatherer produces metrics with embedded quantiles, not separate labeled metrics
	expectedMetrics := map[string]*metric.MetricFamily{
		"test_counter": {
			Name: "test_counter",
			Type: metric.MetricTypeCounter,
			Metrics: []metric.Metric{{
				Value: metric.MetricValue{Value: 12345},
			}},
		},
		"test_counter_float64": {
			Name: "test_counter_float64",
			Type: metric.MetricTypeCounter,
			Metrics: []metric.Metric{{
				Value: metric.MetricValue{Value: 1.1},
			}},
		},
		"test_gauge": {
			Name: "test_gauge",
			Type: metric.MetricTypeGauge,
			Metrics: []metric.Metric{{
				Value: metric.MetricValue{Value: 23456},
			}},
		},
		"test_gauge_float64": {
			Name: "test_gauge_float64",
			Type: metric.MetricTypeGauge,
			Metrics: []metric.Metric{{
				Value: metric.MetricValue{Value: 34567.89},
			}},
		},
		"test_histogram": {
			Name: "test_histogram",
			Type: metric.MetricTypeSummary,
			Metrics: []metric.Metric{{
				Value: metric.MetricValue{
					SampleCount: 0,
					SampleSum:   0,
					Quantiles: []metric.Quantile{
						{Quantile: 0.5, Value: 0},
						{Quantile: 0.75, Value: 0},
						{Quantile: 0.95, Value: 0},
						{Quantile: 0.99, Value: 0},
						{Quantile: 0.999, Value: 0},
						{Quantile: 0.9999, Value: 0},
					},
				},
			}},
		},
		"test_meter": {
			Name: "test_meter",
			Type: metric.MetricTypeGauge,
			Metrics: []metric.Metric{{
				Value: metric.MetricValue{Value: 9999999},
			}},
		},
		"test_timer": {
			Name: "test_timer",
			Type: metric.MetricTypeSummary,
			Metrics: []metric.Metric{{
				Value: metric.MetricValue{
					SampleCount: 6,
					SampleSum:   230000000, // 20+21+22+120+23+24 ms in nanoseconds
					Quantiles: []metric.Quantile{
						{Quantile: 0.5, Value: 22500000},  // ~22.5ms
						{Quantile: 0.75, Value: 48000000}, // ~48ms (interpolated)
						{Quantile: 0.95, Value: 120000000},
						{Quantile: 0.99, Value: 120000000},
						{Quantile: 0.999, Value: 120000000},
						{Quantile: 0.9999, Value: 120000000},
					},
				},
			}},
		},
		"test_resetting_timer": {
			Name: "test_resetting_timer",
			Type: metric.MetricTypeSummary,
			Metrics: []metric.Metric{{
				Value: metric.MetricValue{
					SampleCount: 1,
					SampleSum:   1000000000, // 1 second in nanoseconds
					Quantiles: []metric.Quantile{
						{Quantile: 50, Value: 1000000000},
						{Quantile: 95, Value: 1000000000},
						{Quantile: 99, Value: 1000000000},
					},
				},
			}},
		},
	}

	// Note: empty_resetting_timer and empty_resetting_timer_snapshot are skipped
	// because they have zero count

	assert.Len(t, families, len(expectedMetrics))
	for _, got := range families {
		want, ok := expectedMetrics[got.Name]
		require.True(t, ok, "unexpected metric family: %s", got.Name)
		assert.Equal(t, want.Type, got.Type, "type mismatch for %s", got.Name)
		assert.Equal(t, want.Help, got.Help, "help mismatch for %s", got.Name)

		// For summary types, compare structure but allow for timing variations
		if want.Type == metric.MetricTypeSummary {
			require.Len(t, got.Metrics, 1, "expected 1 metric for %s", got.Name)
			require.Len(t, want.Metrics, 1)

			gotVal := got.Metrics[0].Value
			wantVal := want.Metrics[0].Value

			assert.Equal(t, wantVal.SampleCount, gotVal.SampleCount, "sample count mismatch for %s", got.Name)
			assert.Len(t, gotVal.Quantiles, len(wantVal.Quantiles), "quantile count mismatch for %s", got.Name)

			for i, wantQ := range wantVal.Quantiles {
				assert.Equal(t, wantQ.Quantile, gotVal.Quantiles[i].Quantile, "quantile label mismatch for %s[%d]", got.Name, i)
				// Allow some tolerance for timing-based values
				if got.Name == "test_timer" {
					assert.InDelta(t, wantQ.Value, gotVal.Quantiles[i].Value, wantQ.Value*0.1, "quantile value mismatch for %s[%d]", got.Name, i)
				} else {
					assert.Equal(t, wantQ.Value, gotVal.Quantiles[i].Value, "quantile value mismatch for %s[%d]", got.Name, i)
				}
			}
		} else {
			assert.Equal(t, want.Metrics, got.Metrics, "metrics mismatch for %s", got.Name)
		}
	}

	register(t, "unsupported", metrics.NewHealthcheck(nil))
	families, err = gatherer.Gather()
	assert.ErrorIs(t, err, errMetricTypeNotSupported)
	assert.Empty(t, families)
}
