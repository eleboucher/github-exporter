package main

import (
	"log/slog"
	"os"

	"github.com/eleboucher/github-exporter/cmd"
)

func getDefaultLogLevel() slog.Level {
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		return slog.LevelInfo
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		return slog.LevelInfo
	}
	return level
}

func main() {
	opts := &slog.HandlerOptions{
		Level: getDefaultLogLevel(),
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	slog.SetDefault(logger)
	cmd.Execute()
}
