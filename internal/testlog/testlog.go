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
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/luxfi/geth/log"
)

const (
	termTimeFormat = "01-02|15:04:05.000"
)

// T wraps methods from testing.T used by the test logger into an interface.
// It is specified so that unit tests can instantiate the logger with an
// implementation of T which can capture the output of logging statements
// from T.Logf, as this cannot be using testing.T.
type T interface {
	Logf(format string, args ...any)
	Helper()
}

// logger implements log.Logger such that all output goes to the unit test log via
// t.Logf(). All methods in between logger.Trace, logger.Debug, etc. are marked as test
// helpers, so the file and line number in unit test output correspond to the call site
// which emitted the log message.
type logger struct {
	t  T
	l  log.Logger
	mu *sync.Mutex
	h  *bufHandler
}

type bufHandler struct {
	buf   []slog.Record
	attrs []slog.Attr
	level slog.Level
	mu    sync.Mutex
}

func (h *bufHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buf = append(h.buf, r)
	return nil
}

func (h *bufHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.level
}

func (h *bufHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	records := make([]slog.Record, len(h.buf))
	copy(records[:], h.buf[:])
	return &bufHandler{
		buf:   records,
		attrs: append(h.attrs, attrs...),
		level: h.level,
	}
}

func (h *bufHandler) WithGroup(_ string) slog.Handler {
	panic("not implemented")
}

// Logger returns a logger which logs to the unit test log of t.
func Logger(t T, level slog.Level) log.Logger {
	handler := bufHandler{
		buf:   []slog.Record{},
		attrs: []slog.Attr{},
		level: level,
	}
	return &logger{
		t:  t,
		l:  log.NewLogger(&handler),
		mu: new(sync.Mutex),
		h:  &handler,
	}
}

func (l *logger) Handler() slog.Handler {
	return l.l.Handler()
}

func (l *logger) Write(level slog.Level, msg string, ctx ...interface{}) {}

func (l *logger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.l.Enabled(ctx, level)
}

func (l *logger) Trace(msg string, ctx ...interface{}) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Trace(msg, ctx...)
	l.flush()
}

func (l *logger) Log(level slog.Level, msg string, ctx ...interface{}) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Log(level, msg, ctx...)
	l.flush()
}

func (l *logger) Debug(msg string, ctx ...interface{}) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Debug(msg, ctx...)
	l.flush()
}

func (l *logger) Info(msg string, ctx ...interface{}) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Info(msg, ctx...)
	l.flush()
}

func (l *logger) Warn(msg string, ctx ...interface{}) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Warn(msg, ctx...)
	l.flush()
}

func (l *logger) Error(msg string, ctx ...interface{}) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Error(msg, ctx...)
	l.flush()
}

func (l *logger) Crit(msg string, ctx ...interface{}) {
	l.t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.l.Crit(msg, ctx...)
	l.flush()
}

func (l *logger) With(ctx ...interface{}) log.Logger {
	newLogger := l.l.With(ctx...)
	return &logger{l.t, newLogger, l.mu, newLogger.Handler().(*bufHandler)}
}

func (l *logger) New(ctx ...interface{}) log.Logger {
	return l.With(ctx...)
}

// terminalFormat formats a message similarly to the NewTerminalHandler in the log package.
// The difference is that terminalFormat does not escape messages/attributes and does not pad attributes.
func (h *bufHandler) terminalFormat(r slog.Record) string {
	buf := &bytes.Buffer{}
	lvl := log.LevelAlignedString(r.Level)
	attrs := []slog.Attr{}
	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})

	attrs = append(h.attrs, attrs...)

	fmt.Fprintf(buf, "%s[%s] %s ", lvl, r.Time.Format(termTimeFormat), r.Message)
	if length := len(r.Message); length < 40 {
		buf.Write(bytes.Repeat([]byte{' '}, 40-length))
	}

	for _, attr := range attrs {
		fmt.Fprintf(buf, " %s=%s", attr.Key, string(log.FormatSlogValue(attr.Value, nil)))
	}
	buf.WriteByte('\n')
	return buf.String()
}

// flush writes all buffered messages and clears the buffer.
func (l *logger) flush() {
	l.t.Helper()
	l.h.mu.Lock()
	defer l.h.mu.Unlock()
	for _, r := range l.h.buf {
		l.t.Logf("%s", l.h.terminalFormat(r))
	}
	l.h.buf = nil
}
