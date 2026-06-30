// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
// See the file LICENSE for licensing terms.
package gatherer

// Registry is a narrower interface of a metrics registry containing
// only the required functions for the [Gatherer].
type Registry interface {
	// Call the given function for each registered metric.
	Each(func(string, any))
	// Get the metric by the given name or nil if none is registered.
	Get(string) any
}
