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

// UpsertBatch - пакетная вставка с деградацией до поштучной при ошибке (Batch Degradation).
func (m *SegmentationModel) UpsertBatch(segments []Segmentation) error {
	if len(segments) == 0 {
		return nil
	}

	// 1. Пробуем быстрый batch insert
	err := m.batchInsert(segments)
	if err == nil {
		return nil
	}

	// 2. Деградация: вставляем по одной
	for _, segment := range segments {
		if err := m.singleUpsert(segment); err != nil {
			// Логируем ошибку, но продолжаем с другими записями
			fmt.Printf("Warning: failed to upsert single row (SAP ID: %s): %v\n", segment.AddressSapID, err)
			continue
		}
	}

	return nil
}

// batchInsert - массовая вставка одним запросом.
func (m *SegmentationModel) batchInsert(segments []Segmentation) error {
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

// singleUpsert - вставка одной записи.
func (m *SegmentationModel) singleUpsert(segment Segmentation) error {
	query := `
        INSERT INTO segmentation (address_sap_id, adr_segment, segment_id)
        VALUES (:address_sap_id, :adr_segment, :segment_id)
        ON CONFLICT (address_sap_id)
        DO UPDATE SET
            adr_segment = EXCLUDED.adr_segment,
            segment_id = EXCLUDED.segment_id
    `

	_, err := m.DB.NamedExec(query, segment)
	return err
}
