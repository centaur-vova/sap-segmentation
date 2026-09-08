// internal/importer/importer_test.go
package importer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/centaur-vova/sap-segmentation/internal/config"
	"github.com/centaur-vova/sap-segmentation/internal/model"
)

type MockModel struct {
	upsertFunc func(segments []model.Segmentation) error
}

func (m *MockModel) UpsertBatch(segments []model.Segmentation) error {
	if m.upsertFunc != nil {
		return m.upsertFunc(segments)
	}
	return nil
}

func TestNewImporter(t *testing.T) {
	cfg := &config.Config{
		ConnTimeout: 5,
	}

	imp := NewImporter(cfg, slog.Default(), nil)

	if imp == nil {
		t.Fatal("NewImporter returned nil")
	}

	if imp.client == nil {
		t.Error("HTTP client is nil")
	}

	if imp.client.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", imp.client.Timeout)
	}
}

func TestRunWithData(t *testing.T) {
	callCount := 0

	// Мок API с одной пачкой данных
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		w.Header().Set("Content-Type", "application/json")

		// Первый вызов - данные, второй - пусто
		if callCount == 1 {
			response := `{"items": [
        		{"address_sap_id": "SAP-001", "adr_segment": "SEG-1", "segment_id": 1001},
        		{"address_sap_id": "SAP-002", "adr_segment": "SEG-2", "segment_id": 1002}
    		]}`
			if _, err := w.Write([]byte(response)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		} else {
			if _, err := w.Write([]byte(`{"items": []}`)); err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      5,
		ConnInterval:     10,
		ImportBatchSize:  50,
	}

	// Мок модель
	mockModel := &MockModel{
		upsertFunc: func(segments []model.Segmentation) error {
			if len(segments) != 2 {
				t.Errorf("Expected 2 segments, got %d", len(segments))
			}
			return nil
		},
	}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		model:  mockModel,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	err := imp.Run(context.Background())
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
}

func TestRunWithDBError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := `{"items": [
			{"address_sap_id": "SAP-001", "adr_segment": "SEG-1", "segment_id": 1001}
		]}`
		if _, err := w.Write([]byte(response)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      5,
		ConnInterval:     10,
		ImportBatchSize:  50,
	}

	// Мок модель с ошибкой
	mockModel := &MockModel{
		upsertFunc: func(segments []model.Segmentation) error {
			return errors.New("database error")
		},
	}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		model:  mockModel,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	err := imp.Run(context.Background())
	if err == nil {
		t.Error("Expected error from Run, got nil")
	}
}

func TestRunWithContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel() // Отменяем контекст прямо в момент запроса
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"items": [{"address_sap_id": "SAP-001", "adr_segment": "SEG-1", "segment_id": 1001}]}`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer ts.Close()

	cfg := &config.Config{
		ConnURI:          ts.URL,
		ConnAuthLoginPwd: "test:test",
		ConnUserAgent:    "test-agent",
		ConnTimeout:      5,
		ConnInterval:     10,
		ImportBatchSize:  50,
	}

	mockModel := &MockModel{}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		model:  mockModel,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	err := imp.Run(ctx)
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got: %v", err)
	}
}

func TestFetchData(t *testing.T) {
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

	data, err := imp.fetchData(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("fetchData failed: %v", err)
	}

	if len(data.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(data.Items))
	}

	if data.Items[0].AddressSapID != "SAP-001" {
		t.Errorf("Expected SAP-001, got %s", data.Items[0].AddressSapID)
	}

	if data.Items[0].AdrSegment != "SEG-1" {
		t.Errorf("Expected SEG-1, got %s", data.Items[0].AdrSegment)
	}

	if data.Items[0].SegmentID != 1001 {
		t.Errorf("Expected 1001, got %d", data.Items[0].SegmentID)
	}
}

func TestFetchDataError(t *testing.T) {
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

	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected error to contain '500', got: %v", err)
	}
}

func TestFetchDataWithBadStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Request", http.StatusBadRequest)
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
		t.Error("Expected error for 400 status, got nil")
	}

	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected error to contain '400', got: %v", err)
	}
}

func TestFetchDataWithInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{invalid json`)); err != nil {
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

	_, err := imp.fetchData(context.Background(), ts.URL)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
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
		{
			AddressSapID: "SAP-002",
			AdrSegment:   "SEG-2",
			SegmentID:    1002,
		},
	}

	segments := imp.convertToModel(items)

	if len(segments) != 2 {
		t.Fatalf("Expected 2 segments, got %d", len(segments))
	}

	if segments[0].AddressSapID != "SAP-001" {
		t.Errorf("Expected SAP-001, got %s", segments[0].AddressSapID)
	}

	if segments[0].SegmentID != 1001 {
		t.Errorf("Expected 1001, got %d", segments[0].SegmentID)
	}

	if segments[1].AddressSapID != "SAP-002" {
		t.Errorf("Expected SAP-002, got %s", segments[1].AddressSapID)
	}
}

func TestConvertToModelEmpty(t *testing.T) {
	imp := &Importer{}

	segments := imp.convertToModel([]ERPItem{})

	if len(segments) != 0 {
		t.Errorf("Expected 0 segments, got %d", len(segments))
	}
}

func TestRunEmptyResponse(t *testing.T) {
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
		ConnInterval:     10,
		ImportBatchSize:  50,
	}

	// Используем MockModel вместо реальной модели
	mockModel := &MockModel{
		upsertFunc: func(segments []model.Segmentation) error {
			t.Error("UpsertBatch should not be called on empty API response")
			return nil
		},
	}

	imp := &Importer{
		config: cfg,
		logger: slog.Default(),
		model:  mockModel,
		client: &http.Client{Timeout: 5 * time.Second},
	}

	err := imp.Run(context.Background())
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
}

func TestFetchDataTimeout(t *testing.T) {
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
		ConnTimeout:      1,
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
