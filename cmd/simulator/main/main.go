// Copyright (C) 2019-2025, Lux Industries, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/luxfi/coreth/cmd/simulator/config"
	"github.com/luxfi/coreth/cmd/simulator/load"
	"github.com/luxfi/geth/log"
	"github.com/spf13/pflag"
)

func main() {
	fs := config.BuildFlagSet()
	v, err := config.BuildViper(fs, os.Args[1:])
	if errors.Is(err, pflag.ErrHelp) {
		os.Exit(0)
	}

	if err != nil {
		fmt.Printf("couldn't build viper: %s\n", err)
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("couldn't configure flags: %s\n", err)
		os.Exit(1)
	}

	if v.GetBool(config.VersionKey) {
		fmt.Printf("%s\n", config.Version)
		os.Exit(0)
	}

	logLevel, err := lvlFromString(v.GetString(config.LogLevelKey))
	if err != nil {
		fmt.Printf("couldn't parse log level: %s\n", err)
		os.Exit(1)
	}
	gethLogger := log.NewLogger(log.NewTerminalHandlerWithLevel(os.Stderr, logLevel, true))
	log.SetDefault(gethLogger)

	config, err := config.BuildConfig(v)
	if err != nil {
		fmt.Printf("%s\n", err)
		os.Exit(1)
	}
	if err := load.ExecuteLoader(context.Background(), config); err != nil {
		fmt.Printf("load execution failed: %s\n", err)
		os.Exit(1)
	}
}

// lvlFromString converts a log level string to slog.Level
func lvlFromString(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "trace", "trce":
		return log.LevelTrace, nil
	case "debug", "dbug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error", "eror":
		return slog.LevelError, nil
	case "crit":
		return log.LevelCrit, nil
	default:
		return slog.LevelDebug, fmt.Errorf("unknown level: %s", s)
	}
}
