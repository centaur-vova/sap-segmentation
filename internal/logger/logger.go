// internal/logger/logger.go
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/centaur-vova/sap-segmentation/internal/config"
)

type Logger struct {
	*slog.Logger
	file *os.File
}

func NewLogger(cfg *config.Config) (*Logger, error) {
	// Создаем директорию для логов
	if err := os.MkdirAll(cfg.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	logPath := filepath.Join(cfg.LogDir, cfg.LogFile)

	// Открываем файл для логирования
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Мультиплексируем вывод: консоль + файл
	multiWriter := io.MultiWriter(os.Stdout, file)

	// Настраиваем уровень логирования
	level := parseLevel(cfg.LogLevel)

	// Создаем handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(multiWriter, opts)
	} else {
		// JSON по умолчанию для production
		handler = slog.NewJSONHandler(multiWriter, opts)
	}

	logger := slog.New(handler)

	// Очистка старых логов
	if err := cleanupOldLogs(cfg.LogDir, cfg.LogCleanupMaxAge, logger); err != nil {
		logger.Warn("Failed to cleanup old logs", "error", err)
	}

	return &Logger{
		Logger: logger,
		file:   file,
	}, nil
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func cleanupOldLogs(logDir string, maxAgeDays int, logger *slog.Logger) error {
	files, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Удаляем только .log файлы
		if filepath.Ext(file.Name()) != ".log" {
			continue
		}

		filePath := filepath.Join(logDir, file.Name())
		fileInfo, err := file.Info()
		if err != nil {
			logger.Error("Failed to get file info", "file", file.Name(), "error", err)
			continue
		}

		if fileInfo.ModTime().Before(cutoff) {
			if err := os.Remove(filePath); err != nil {
				logger.Error("Failed to delete old log file", "file", file.Name(), "error", err)
			} else {
				logger.Info("Removed old log file",
					"file", file.Name(),
					"modified", fileInfo.ModTime().Format(time.RFC3339),
				)
			}
		}
	}

	return nil
}
