// internal/model/segmentation_test.go
package model

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestUpsertBatchSuccess(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() {
		mock.ExpectClose()
		if err := mockDB.Close(); err != nil {
			t.Errorf("Failed to close mock DB: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	model := NewSegmentationModel(sqlxDB)

	segments := []Segmentation{
		{AddressSapID: "SAP-001", AdrSegment: "SEG-1", SegmentID: 1001},
		{AddressSapID: "SAP-002", AdrSegment: "SEG-2", SegmentID: 1002},
	}

	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("SAP-001", "SEG-1", int64(1001), "SAP-002", "SEG-2", int64(1002)).
		WillReturnResult(sqlmock.NewResult(2, 2))

	err = model.UpsertBatch(segments)
	if err != nil {
		t.Errorf("UpsertBatch failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestUpsertBatchDegradation(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() {
		mock.ExpectClose()
		if err := mockDB.Close(); err != nil {
			t.Errorf("Failed to close mock DB: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	model := NewSegmentationModel(sqlxDB)

	segments := []Segmentation{
		{AddressSapID: "SAP-001", AdrSegment: "SEG-1", SegmentID: 1001},
		{AddressSapID: "SAP-002", AdrSegment: "SEG-2", SegmentID: 1002},
	}

	// Batch insert падает с ошибкой
	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("SAP-001", "SEG-1", int64(1001), "SAP-002", "SEG-2", int64(1002)).
		WillReturnError(errors.New("batch insert failed"))

	// Деградация: поштучная вставка
	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("SAP-001", "SEG-1", int64(1001)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("SAP-002", "SEG-2", int64(1002)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = model.UpsertBatch(segments)
	if err != nil {
		t.Errorf("UpsertBatch should not fail on degradation: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestUpsertBatchDegradationWithBadRow(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer func() {
		mock.ExpectClose()
		if err := mockDB.Close(); err != nil {
			t.Errorf("Failed to close mock DB: %v", err)
		}
	}()

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	model := NewSegmentationModel(sqlxDB)

	segments := []Segmentation{
		{AddressSapID: "SAP-001", AdrSegment: "SEG-1", SegmentID: 1001},
		{AddressSapID: "", AdrSegment: "SEG-2", SegmentID: 1002}, // Битая запись (пустой ID)
		{AddressSapID: "SAP-003", AdrSegment: "SEG-3", SegmentID: 1003},
	}

	// Batch insert падает
	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("SAP-001", "SEG-1", int64(1001), "", "SEG-2", int64(1002), "SAP-003", "SEG-3", int64(1003)).
		WillReturnError(errors.New("batch insert failed"))

	// Поштучная вставка
	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("SAP-001", "SEG-1", int64(1001)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Битая запись падает
	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("", "SEG-2", int64(1002)).
		WillReturnError(errors.New("empty address_sap_id"))

	// Третья запись успешна
	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("SAP-003", "SEG-3", int64(1003)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = model.UpsertBatch(segments)
	if err != nil {
		t.Errorf("UpsertBatch should not fail even with bad row: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

func TestUpsertBatchEmpty(t *testing.T) {
	model := NewSegmentationModel(nil)

	err := model.UpsertBatch([]Segmentation{})
	if err != nil {
		t.Errorf("UpsertBatch with empty slice should return nil, got %v", err)
	}
}
