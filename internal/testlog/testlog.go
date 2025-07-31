// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package testlog provides a log handler for unit tests.
package testlog

import (
	"log/slog"

	"github.com/luxfi/log"
)

// T wraps methods from testing.T used by the test logger into an interface.
// It is specified so that unit tests can instantiate the logger with an
// implementation of T which can capture the output of logging statements
// from T.Logf, as this cannot be using testing.T.
type T interface {
	Logf(format string, args ...any)
	Helper()
}

// Logger returns a logger which logs to the unit test log of t.
func Logger(t T, level slog.Level) log.Logger {
	// For now, just use a no-op logger for tests
	// TODO: Implement proper test logging with luxfi/log
	return log.NewNoOpLogger()
}

