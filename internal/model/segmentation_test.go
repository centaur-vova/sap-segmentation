// internal/model/segmentation_test.go
package model

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func TestUpsertBatch(t *testing.T) {
	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}

	sqlxDB := sqlx.NewDb(mockDB, "sqlmock")
	model := NewSegmentationModel(sqlxDB)

	segments := []Segmentation{
		{
			AddressSapID: "SAP-001",
			AdrSegment:   "SEG-001",
			SegmentID:    1001,
		},
	}

	// Ожидаем начало транзакции
	mock.ExpectBegin()

	// Ожидаем Exec внутри транзакции
	mock.ExpectExec("INSERT INTO segmentation").
		WithArgs("SAP-001", "SEG-001", int64(1001)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Ожидаем Commit
	mock.ExpectCommit()

	err = model.UpsertBatch(segments)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}

	mock.ExpectClose()
	if err := mockDB.Close(); err != nil {
		t.Errorf("Failed to close mock DB: %v", err)
	}
}
