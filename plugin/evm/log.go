// (c) 2019-2020, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package evm

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/luxfi/geth/log"
	gethlog "github.com/ethereum/go-ethereum/log"
	"golang.org/x/exp/slog"
)

type GethLogger struct {
	gethlog.Logger

	logLevel *slog.LevelVar
}

// InitLogger initializes logger with alias and sets the log level and format with the original [os.StdErr] interface
// along with the context logger.
func InitLogger(alias string, level string, jsonFormat bool, writer io.Writer) (GethLogger, error) {
	logLevel := &slog.LevelVar{}

	var handler slog.Handler
	if jsonFormat {
		chainStr := fmt.Sprintf("%s Chain", alias)
		handler = log.JSONHandlerWithLevel(writer, logLevel)
		handler = &addContext{Handler: handler, logger: chainStr}
	} else {
		useColor := false
		chainStr := fmt.Sprintf("<%s Chain> ", alias)
		termHandler := log.NewTerminalHandlerWithLevel(writer, logLevel, useColor)
		termHandler.Prefix = func(r slog.Record) string {
			file, line := getSource(r)
			if file != "" {
				return fmt.Sprintf("%s%s:%d ", chainStr, file, line)
			}
			return chainStr
		}
		handler = termHandler
	}

	// Create handler by wrapping the exp/slog handler with our adapter
	adaptedHandler := &slogAdapter{handler: handler}
	c := GethLogger{
		Logger:   gethlog.NewLogger(adaptedHandler),
		logLevel: logLevel,
	}

	if err := c.SetLogLevel(level); err != nil {
		return GethLogger{}, err
	}
	gethlog.SetDefault(c.Logger)
	return c, nil
}

// SetLogLevel sets the log level of initialized log handler.
func (c *GethLogger) SetLogLevel(level string) error {
	// Set log level
	logLevel, err := log.LvlFromString(level)
	if err != nil {
		return err
	}
	// Convert exp/slog.Level to int value for standard library slog.Level
	c.logLevel.Set(slog.Level(int(logLevel)))
	return nil
}

// locationTrims are trimmed for display to avoid unwieldy log lines.
var locationTrims = []string{
	"geth",
}

func trimPrefixes(s string) string {
	for _, prefix := range locationTrims {
		idx := strings.LastIndex(s, prefix)
		if idx < 0 {
			continue
		}
		slashIdx := strings.Index(s[idx:], "/")
		if slashIdx < 0 || slashIdx+idx >= len(s)-1 {
			continue
		}
		s = s[idx+slashIdx+1:]
	}
	return s
}

func getSource(r slog.Record) (string, int) {
	frames := runtime.CallersFrames([]uintptr{r.PC})
	frame, _ := frames.Next()
	return trimPrefixes(frame.File), frame.Line
}

type addContext struct {
	slog.Handler

	logger string
}

func (a *addContext) Handle(ctx context.Context, r slog.Record) error {
	r.Add(slog.String("logger", a.logger))
	file, line := getSource(r)
	if file != "" {
		r.Add(slog.String("caller", fmt.Sprintf("%s:%d", file, line)))
	}
	return a.Handler.Handle(ctx, r)
}
