// internal/model/segmentation.go
package model

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

// Segmentation - структура для таблицы segmentation.
type Segmentation struct {
	ID           int64  `db:"id"`
	AddressSapID string `db:"address_sap_id"`
	AdrSegment   string `db:"adr_segment"`
	SegmentID    int64  `db:"segment_id"`
}

// SegmentationModel - модель для работы с таблицей segmentation.
type SegmentationModel struct {
	DB *sqlx.DB
}

// NewSegmentationModel - создает новый экземпляр SegmentationModel.
func NewSegmentationModel(db *sqlx.DB) *SegmentationModel {
	return &SegmentationModel{DB: db}
}

// BatchInsert для массовой вставки.
func (m *SegmentationModel) BatchInsert(segments []Segmentation) error {
	if len(segments) == 0 {
		return nil
	}

	// Создаем batch insert
	valueStrings := make([]string, 0, len(segments))
	valueArgs := make([]interface{}, 0, len(segments)*3)

	for i, seg := range segments {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d)", i*3+1, i*3+2, i*3+3))
		valueArgs = append(valueArgs, seg.AddressSapID, seg.AdrSegment, seg.SegmentID)
	}

	query := fmt.Sprintf(`
        INSERT INTO segmentation (address_sap_id, adr_segment, segment_id)
        VALUES %s
        ON CONFLICT (address_sap_id)
        DO UPDATE SET
            adr_segment = EXCLUDED.adr_segment,
            segment_id = EXCLUDED.segment_id
    `, strings.Join(valueStrings, ","))

	_, err := m.DB.Exec(query, valueArgs...)
	return err
}

// UpsertBatch - для одиночных вставок с транзакцией.
func (m *SegmentationModel) UpsertBatch(segments []Segmentation) error {
	if len(segments) == 0 {
		return nil
	}

	query := `
        INSERT INTO segmentation (address_sap_id, adr_segment, segment_id)
        VALUES (:address_sap_id, :adr_segment, :segment_id)
        ON CONFLICT (address_sap_id)
        DO UPDATE SET
            adr_segment = EXCLUDED.adr_segment,
            segment_id = EXCLUDED.segment_id
    `

	tx, err := m.DB.Beginx()
	if err != nil {
		return err
	}
	defer func() {
		// Rollback после Commit вернет ошибку sql.ErrTxDone - это нормально
		_ = tx.Rollback()
	}()

	for _, segment := range segments {
		if _, err := tx.NamedExec(query, segment); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}
