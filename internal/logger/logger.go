// internal/logger/logger.go
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/centaur-vova/sap-segmentation/internal/config"
)

// Logger - обертка над slog.Logger с поддержкой записи в файл.
type Logger struct {
	*slog.Logger
	file *os.File
}

// NewLogger - создает логгер с выводом в консоль и файл.
// NewLogger - создает логгер с выводом в консоль и файл.
func NewLogger(cfg *config.Config) (*Logger, error) {
	var writers []io.Writer
	var file *os.File

	if cfg.LogToConsole {
		writers = append(writers, os.Stdout)
	}

	if cfg.LogToFile {
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

		writers = append(writers, file)
	}

	// Если ни один вывод не настроен - используем stdout
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	// Мультиплексируем вывод
	multiWriter := io.MultiWriter(writers...)

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
		logger.Error("Failed to cleanup old logs on startup", "error", err)
	}

	return &Logger{
		Logger: logger,
		file:   file,
	}, nil
}

// Close - закрывает файл лога.
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
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
		return fmt.Errorf("failed to read log dir: %w", err)
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)

	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".log" {
			continue
		}

		filePath := filepath.Join(logDir, file.Name())
		fileInfo, err := file.Info()
		if err != nil {
			continue
		}

		// Не удаляем файлы моложе cutoff
		if fileInfo.ModTime().After(cutoff) {
			continue
		}

		if err := os.Remove(filePath); err != nil {
			logger.Error("Failed to delete old log file", "file", file.Name(), "error", err)
		} else {
			logger.Info("Removed old log file", "file", file.Name(), "modified", fileInfo.ModTime().Format(time.RFC3339))
		}
	}

	return nil
}
