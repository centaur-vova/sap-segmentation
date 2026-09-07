// internal/importer/importer_test.go
package importer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/centaur-vova/sap-segmentation/internal/config"
	"github.com/centaur-vova/sap-segmentation/internal/model"
)

func TestFetchData(t *testing.T) {
	// Тестовые данные
	expectedItems := []ERPItem{
		{
			AddressSapID: "SAP-001",
			AdrSegment:   "SEG-1",
			SegmentID:    1001,
		},
		{
			AddressSapID: "SAP-002",
			AdrSegment:   "SEG-2",
			SegmentID:    1002,
		},
	}

	// Создаем тестовый сервер
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем метод
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET, got %s", r.Method)
		}

		// Проверяем User-Agent
		if r.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("Expected User-Agent 'test-agent', got '%s'", r.Header.Get("User-Agent"))
		}

		// Проверяем Basic Auth
		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("test:test"))
		if r.Header.Get("Authorization") != expectedAuth {
			t.Errorf("Expected Authorization '%s', got '%s'", expectedAuth, r.Header.Get("Authorization"))
		}

		// Отправляем ответ
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(APIResponse{Items: expectedItems}); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer ts.Close()

	// Создаем конфиг
	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      5,
	}

	// Создаем импортер (без БД для этого теста)
	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		client: &http.Client{Timeout: 5 * time.Second},
	}

	// Тестируем
	data, err := imp.fetchData(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchData failed: %v", err)
	}

	// Проверяем результат
	if len(data.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(data.Items))
	}

	if data.Items[0].AddressSapID != "SAP-001" {
		t.Errorf("Expected SAP-001, got %s", data.Items[0].AddressSapID)
	}
}

func TestFetchDataError(t *testing.T) {
	// Тест на 500 ошибку
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

	_, err := imp.fetchData(context.Background(), ts.URL)
	if err == nil {
		t.Error("Expected error for 500 status, got nil")
	}
}

func TestConvertToModel(t *testing.T) {
	imp := &Importer{}

	items := []ERPItem{
		{
			AddressSapID: "SAP-001",
			AdrSegment:   "SEG-1",
			SegmentID:    1001,
		},
	}

	segments := imp.convertToModel(items)

	if len(segments) != 1 {
		t.Fatalf("Expected 1 segment, got %d", len(segments))
	}

	if segments[0].AddressSapID != "SAP-001" {
		t.Errorf("Expected SAP-001, got %s", segments[0].AddressSapID)
	}

	if segments[0].SegmentID != 1001 {
		t.Errorf("Expected 1001, got %d", segments[0].SegmentID)
	}
}

func TestRunEmptyResponse(t *testing.T) {
	// Тест на пустой ответ
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
		ConnInterval:     10, // 10ms для быстрого теста
		ImportBatchSize:  50,
	}

	// Создаем мок для модели
	mockModel := &model.SegmentationModel{DB: nil} // Для теста Run без БД

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		model:  mockModel,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	ctx := context.Background()
	err := imp.Run(ctx)

	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
}

func TestFetchDataTimeout(t *testing.T) {
	// Тест на таймаут
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		if _, err := w.Write([]byte(`{"items": []}`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      1, // 1 секунда - меньше чем задержка в 2 секунды
	}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		client: &http.Client{Timeout: 1 * time.Second},
	}

	_, err := imp.fetchData(context.Background(), ts.URL)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}
