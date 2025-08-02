// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"

	luxlog "github.com/luxfi/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	luxlog.Logger

	logLevel *slog.LevelVar
}

// InitLogger initializes logger with alias and sets the log level and format with the original [os.StdErr] interface
// along with the context logger.
func InitLogger(alias string, level string, jsonFormat bool, writer io.Writer) (Logger, error) {
	logLevel := &slog.LevelVar{}

	// Convert log level to zap level
	var zapLevel zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		zapLevel = zapcore.DebugLevel
		logLevel.Set(slog.LevelDebug)
	case "info":
		zapLevel = zapcore.InfoLevel
		logLevel.Set(slog.LevelInfo)
	case "warn":
		zapLevel = zapcore.WarnLevel
		logLevel.Set(slog.LevelWarn)
	case "error":
		zapLevel = zapcore.ErrorLevel
		logLevel.Set(slog.LevelError)
	default:
		return Logger{}, fmt.Errorf("unknown log level: %s", level)
	}

	// Create zap encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "source",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller: func(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
			chainStr := fmt.Sprintf("<%s Chain> ", alias)
			enc.AppendString(fmt.Sprintf("%s%s:%d", chainStr, trimPrefixes(caller.File), caller.Line))
		},
	}

	// Create encoder
	var encoder zapcore.Encoder
	if jsonFormat {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create core
	core := zapcore.NewCore(encoder, zapcore.AddSync(writer), zapLevel)
	
	// Create zap logger
	zapLogger := zap.New(core, zap.AddCaller())
	
	// Create luxfi logger
	luxLogger := luxlog.NewZapLogger(zapLogger)

	c := Logger{
		Logger:   luxLogger,
		logLevel: logLevel,
	}

	// Set as global logger
	luxlog.SetGlobalLogger(luxLogger)
	
	return c, nil
}

// SetLogLevel sets the log level of initialized log handler.
func (l *Logger) SetLogLevel(level string) error {
	// Set log level for slog
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return fmt.Errorf("unknown log level: %s", level)
	}
	l.logLevel.Set(slogLevel)
	
	// Note: luxfi/log doesn't expose a SetLevel method on individual loggers
	// The level is controlled at creation time via the zap configuration
	return nil
}

// locationTrims are trimmed for display to avoid unwieldy log lines.
var locationTrims = []string{
	"coreth",
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
