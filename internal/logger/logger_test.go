package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/centaur-vova/sap-segmentation/internal/config"
)

func TestNewLogger(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		LogDir:           tmpDir,
		LogFile:          "test.log",
		LogToConsole:     false,
		LogToFile:        true,
		LogLevel:         "debug",
		LogFormat:        "text",
		LogCleanupMaxAge: 7,
	}

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer log.Close()

	// Пишем в лог
	log.Info("test")

	// Проверяем что файл существует после записи
	logPath := filepath.Join(tmpDir, "test.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file was not created")
	}
}

func TestNewLoggerNoOutputs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		LogDir:       tmpDir,
		LogFile:      "test.log",
		LogToConsole: false,
		LogToFile:    false,
		LogLevel:     "info",
		LogFormat:    "json",
	}

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer log.Close()

	// Должен работать даже без выводов (stdout по умолчанию)
	log.Info("test")
}

func TestNewLoggerWithTextFormat(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		LogDir:       tmpDir,
		LogFile:      "test.log",
		LogToConsole: false,
		LogToFile:    true,
		LogLevel:     "debug",
		LogFormat:    "text",
	}

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer log.Close()

	log.Debug("debug message")
	log.Info("info message")
	log.Warn("warn message")
	log.Error("error message")
}

func TestNewLoggerWithConsoleOutput(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		LogDir:       tmpDir,
		LogFile:      "test.log",
		LogToConsole: true,
		LogToFile:    true,
		LogLevel:     "info",
		LogFormat:    "json",
	}

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}
	defer log.Close()

	log.Info("test with console")
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		LogDir:       tmpDir,
		LogFile:      "test.log",
		LogToConsole: false,
		LogToFile:    true,
		LogLevel:     "info",
		LogFormat:    "text",
	}

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	if err := log.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestCloseWithoutFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		LogDir:       tmpDir,
		LogFile:      "test.log",
		LogToConsole: true,
		LogToFile:    false,
		LogLevel:     "info",
		LogFormat:    "text",
	}

	log, err := NewLogger(cfg)
	if err != nil {
		t.Fatalf("NewLogger failed: %v", err)
	}

	// Не должно быть ошибки при закрытии без файла
	if err := log.Close(); err != nil {
		t.Errorf("Close without file failed: %v", err)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"invalid", "invalid", slog.LevelInfo},
		{"empty", "", slog.LevelInfo},
		{"uppercase", "INFO", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCleanupOldLogs(t *testing.T) {
	tmpDir := t.TempDir()

	// Создаем старый файл
	oldFile := filepath.Join(tmpDir, "old.log")
	os.WriteFile(oldFile, []byte("test"), 0644)

	// Устанавливаем время модификации 10 дней назад
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldFile, oldTime, oldTime)

	// Создаем новый файл
	newFile := filepath.Join(tmpDir, "new.log")
	os.WriteFile(newFile, []byte("test"), 0644)

	logger := slog.Default()
	cleanupOldLogs(tmpDir, 7, logger)

	// Старый файл должен быть удален
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old log file was not deleted")
	}

	// Новый файл должен остаться
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("New log file was deleted")
	}
}

func TestCleanupOldLogsNoDir(t *testing.T) {
	logger := slog.Default()

	// Несуществующая директория - не должно быть паники
	cleanupOldLogs("/nonexistent/path", 7, logger)
}

func TestCleanupOldLogsNonLogFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Создаем не .log файл
	txtFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(txtFile, []byte("test"), 0644)

	// Устанавливаем старое время
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(txtFile, oldTime, oldTime)

	logger := slog.Default()
	cleanupOldLogs(tmpDir, 7, logger)

	// .txt файл не должен быть удален
	if _, err := os.Stat(txtFile); os.IsNotExist(err) {
		t.Error("Non-log file was deleted")
	}
}
