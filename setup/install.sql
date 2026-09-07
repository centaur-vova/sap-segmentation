-- Создание таблицы segmentation
CREATE TABLE IF NOT EXISTS segmentation (
    id BIGSERIAL PRIMARY KEY,
    address_sap_id VARCHAR(255) NOT NULL UNIQUE,
    adr_segment VARCHAR(16),
    segment_id BIGINT
);

-- Создание индекса для ускорения поиска по address_sap_id
CREATE INDEX IF NOT EXISTS idx_segmentation_address_sap_id ON segmentation(address_sap_id);