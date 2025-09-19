package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Configure instancie un logger structuré basé sur slog avec le niveau demandé.
func Configure(level string) (*slog.Logger, error) {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: parseLevel(level)})
	return slog.New(handler), nil
}

// Writer retourne un io.Writer qui transmet chaque écriture au logger.
func Writer(logger *slog.Logger, level slog.Level) io.Writer {
	return logWriter{logger: logger, level: level}
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type logWriter struct {
	logger *slog.Logger
	level  slog.Level
}

func (w logWriter) Write(p []byte) (int, error) {
	if w.logger == nil {
		return len(p), nil
	}

	message := strings.TrimSpace(string(p))
	if message != "" {
		w.logger.Log(context.Background(), w.level, message)
	}
	return len(p), nil
}
