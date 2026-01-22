package main

import (
	"log/slog"
	"os"

	"github.com/eleboucher/github-exporter/cmd"
)

func main() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	slog.SetDefault(logger)
	cmd.Execute()
}
