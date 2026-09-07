package importer

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/centaur-vova/sap-segmentation/internal/config"
)

func TestFetchWithRetrySuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"items": []}`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      5,
	}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		client: &http.Client{Timeout: 5 * time.Second},
	}

	data, err := imp.fetchWithRetry(context.Background(), ts.URL, maxRetries)
	if err != nil {
		t.Fatalf("fetchWithRetry failed: %v", err)
	}

	if data == nil {
		t.Error("Expected data, got nil")
	}
}

func TestFetchWithRetrySuccessAfterFailures(t *testing.T) {
	attempts := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++

		// Первые 2 запроса - ошибка
		if attempts <= 2 {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Третий запрос - успех
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"items": []}`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      5,
	}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		client: &http.Client{Timeout: 5 * time.Second},
	}

	data, err := imp.fetchWithRetry(context.Background(), ts.URL, maxRetries)
	if err != nil {
		t.Fatalf("fetchWithRetry failed: %v", err)
	}

	if data == nil {
		t.Error("Expected data, got nil")
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestFetchWithRetryAllFailures(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      5,
	}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		client: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := imp.fetchWithRetry(context.Background(), ts.URL, maxRetries)
	if err == nil {
		t.Error("Expected error, got nil")
	}

	if !strings.Contains(err.Error(), "max retries exceeded") {
		t.Errorf("Expected error to contain 'max retries exceeded', got: %v", err)
	}
}

func TestFetchWithRetryContextCancelled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      5,
	}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		client: &http.Client{Timeout: 5 * time.Second},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Отменяем сразу

	_, err := imp.fetchWithRetry(ctx, ts.URL, maxRetries)
	if err == nil {
		t.Error("Expected context error, got nil")
	}
}
